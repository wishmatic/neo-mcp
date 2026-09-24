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
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/resolve"
)

func checkerPNG(t *testing.T, size int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := range size {
		for x := range size {
			alpha := uint8(0)
			if x < size/2 {
				alpha = 0xff
			}

			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 40, B: 10, A: alpha})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func TestBgSchema(t *testing.T) {
	s := bgSchema(imgfmt.Default)

	for _, field := range []string{"hex", "image_url"} {
		if !slices.Contains(s.Required, field) {
			t.Errorf("bg: %q is not required", field)
		}
	}

	format := s.Properties["format"]
	if format == nil {
		t.Fatal("bg: format property is missing")
	}

	if len(format.Enum) != len(imgfmt.Names()) {
		t.Errorf("bg: format has %d enum values, want %d", len(format.Enum), len(imgfmt.Names()))
	}
}

func TestBgImageSizedToInput(t *testing.T) {
	base := color.NRGBA{R: 0x4a, G: 0x6f, B: 0xa5, A: 0xff}

	out, err := bgImage(checkerPNG(t, 12), base, imgfmt.PNG)
	if err != nil {
		t.Fatalf("bgImage() error: %v", err)
	}

	img, err := imgfmt.Decode(out)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if got, want := img.Bounds(), image.Rect(0, 0, 12, 12); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	if _, _, _, alpha := img.At(11, 11).RGBA(); alpha != 0xffff {
		t.Errorf("alpha behind the transparent half = %d, want an opaque background", alpha>>8)
	}
}

func TestBgImageRejectsUndecodableInput(t *testing.T) {
	if _, err := bgImage([]byte("not an image"), color.NRGBA{A: 0xff}, imgfmt.PNG); err == nil {
		t.Error("bgImage() expected an error, got nil")
	}
}

func TestBgRejectsBadHex(t *testing.T) {
	h := &handlers{log: zapNop()}

	_, _, err := h.bg(context.Background(), nil, bgInput{
		Hex:      "not hex",
		ImageURL: "https://example.com/x.png",
	})
	if err == nil || !strings.HasPrefix(err.Error(), "bg:") {
		t.Fatalf("error = %v, want a bg: prefix", err)
	}
}

func TestBgCallToolReturnsImage(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(checkerPNG(t, 10))
	}))
	t.Cleanup(images.Close)

	resolver, err := resolve.New(nil, "", nil)
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	srv, err := New(Deps{
		Log:          zapNop(),
		Publisher:    newTestPublisher(t),
		Resolver:     resolver,
		OutputFormat: imgfmt.Default,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bg",
		Arguments: map[string]any{
			"hex":       "#4a6fa5",
			"image_url": images.URL + "/x.png",
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

func TestBgCallToolFetchFailure(t *testing.T) {
	resolver, err := resolve.New(nil, "", nil)
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	h := &handlers{log: zapNop(), resolver: resolver, publisher: newTestPublisher(t)}

	_, _, err = h.bg(context.Background(), nil, bgInput{
		Hex:      "#000000",
		ImageURL: "http://127.0.0.1:1/x.png",
	})
	if err == nil || !strings.HasPrefix(err.Error(), "bg:") {
		t.Fatalf("error = %v, want a bg: prefix", err)
	}
}
