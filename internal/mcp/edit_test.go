package mcp

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/edit"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/resolve"
)

func marginPNG(t *testing.T, size, margin int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := margin; y < size-margin; y++ {
		for x := margin; x < size-margin; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func TestEditOptionsAlwaysCrops(t *testing.T) {
	tests := []struct {
		name string
		in   editInput
		want edit.Options
	}{
		{name: "bare crop", in: editInput{}, want: edit.Options{}},
		{
			name: "square defaults the padding",
			in:   editInput{IsSquare: true},
			want: edit.Options{IsSquare: true, Padding: edit.DefaultSquarePadding},
		},
		{
			name: "circle implies square with no padding",
			in:   editInput{IsCircle: true},
			want: edit.Options{IsSquare: true, IsCircle: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editOptions(tt.in); got != tt.want {
				t.Errorf("editOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEditOptionsMapsExplicitPadding(t *testing.T) {
	five := 5

	got := editOptions(editInput{IsSquare: true, IsCircle: true, Padding: &five})
	want := edit.Options{IsSquare: true, IsCircle: true, Padding: 5}

	if got != want {
		t.Errorf("editOptions() = %+v, want %+v", got, want)
	}
}

func TestEditImageTrimsToContent(t *testing.T) {
	out, err := editImage(marginPNG(t, 20, 5), edit.Options{}, format.PNG)
	if err != nil {
		t.Fatalf("editImage() error: %v", err)
	}

	img, err := format.Decode(out)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if bounds := img.Bounds(); bounds.Dx() != 10 || bounds.Dy() != 10 {
		t.Errorf("bounds = %v, want 10x10", bounds)
	}
}

func TestEditImageCircleCutsCorners(t *testing.T) {
	out, err := editImage(marginPNG(t, 20, 2), edit.Options{IsSquare: true, IsCircle: true}, format.PNG)
	if err != nil {
		t.Fatalf("editImage() error: %v", err)
	}

	img, err := format.Decode(out)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	for _, p := range []image.Point{{0, 0}, {15, 0}, {0, 15}, {15, 15}} {
		if _, _, _, alpha := img.At(p.X, p.Y).RGBA(); alpha != 0 {
			t.Errorf("corner %v alpha = %d, want it cut off by the circle", p, alpha>>8)
		}
	}

	if _, _, _, alpha := img.At(8, 8).RGBA(); alpha>>8 != 255 {
		t.Errorf("center alpha = %d, want the content kept", alpha>>8)
	}
}

func TestEditImageErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		opts edit.Options
	}{
		{name: "not an image", data: []byte("not an image")},
		{name: "no visible content", data: marginPNG(t, 8, 4)},
		{name: "negative padding", data: marginPNG(t, 8, 2), opts: edit.Options{Padding: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := editImage(tt.data, tt.opts, format.PNG); err == nil {
				t.Fatal("editImage() expected error, got nil")
			}
		})
	}
}

func TestEditSchema(t *testing.T) {
	s := editSchema(format.Default)

	if !slices.Contains(s.Required, "image_url") {
		t.Error("edit: image_url is not required")
	}

	for _, field := range []string{"square", "circle"} {
		prop := s.Properties[field]
		if prop == nil {
			t.Fatalf("edit: %q property is missing", field)
		}

		if string(prop.Default) != "false" {
			t.Errorf("edit: %q default = %s, want false", field, prop.Default)
		}
	}

	padding := s.Properties["padding"]
	if padding == nil {
		t.Fatal("edit: padding property is missing")
	}

	if padding.Default != nil {
		t.Errorf("edit: padding default = %s, want none", padding.Default)
	}

	property := s.Properties["format"]
	if property == nil {
		t.Fatal("edit: format property is missing")
	}

	if len(property.Enum) != len(format.Names()) {
		t.Errorf("edit: format has %d enum values, want %d", len(property.Enum), len(format.Names()))
	}
}

func TestEditCallToolReturnsImage(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(marginPNG(t, 16, 4))
	}))
	t.Cleanup(images.Close)

	resolver, err := resolve.New(nil, "", nil)
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	srv, err := New(Clients{
		Log:                 zapNop(),
		Store:               newTestStore(t),
		Resolver:            resolver,
		DefaultOutputFormat: format.Default,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "edit",
		Arguments: map[string]any{
			"image_url": images.URL + "/x.png",
			"circle":    true,
			"format":    "png",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	if len(result.Content) != 2 {
		t.Fatalf("content = %d, want a URL and one image", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.HasSuffix(text.Text, ".png") {
		t.Fatalf("content[0] = %#v, want an uploaded .png URL", result.Content[0])
	}

	if _, ok := result.Content[1].(*mcp.ImageContent); !ok {
		t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
	}
}

func TestEditCallToolFetchFailure(t *testing.T) {
	resolver, err := resolve.New(nil, "", nil)
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	h := &Clients{Log: zapNop(), Resolver: resolver, Store: newTestStore(t)}

	_, _, err = h.edit(context.Background(), nil, editInput{ImageURL: "http://127.0.0.1:1/x.png"})
	if err == nil || !strings.HasPrefix(err.Error(), "edit:") {
		t.Fatalf("error = %v, want an edit: prefix", err)
	}
}
