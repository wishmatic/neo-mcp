package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/publish"
)

func TestConvertSchema(t *testing.T) {
	s := convertSchema()

	for _, field := range []string{"image_url", "format"} {
		if !slices.Contains(s.Required, field) {
			t.Errorf("convert: %s is not required", field)
		}
	}

	property := s.Properties["format"]
	if property == nil {
		t.Fatal("convert: format property is missing")
	}

	if property.Default != nil {
		t.Errorf("convert: format default = %s, want none", property.Default)
	}

	if len(property.Enum) != len(format.Names()) {
		t.Errorf("convert: format has %d enum values, want %d", len(property.Enum), len(format.Names()))
	}
}

func TestConvertCallToolReturnsImage(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testImagePNG(t))
	}))
	t.Cleanup(images.Close)

	srv, err := New(Deps{
		Log:       zapNop(),
		Publisher: newTestPublisher(t),
		Resolver:  newResolver(t),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	for _, format := range []format.Format{format.JPEG, format.JXL, format.WebP} {
		t.Run(format.String(), func(t *testing.T) {
			result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
				Name: "convert",
				Arguments: map[string]any{
					"image_url": images.URL + "/x.png",
					"format":    format.String(),
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
			if !ok || !strings.HasSuffix(text.Text, "."+format.Extension()) {
				t.Fatalf("content[0] = %#v, want an uploaded .%s URL", result.Content[0], format.Extension())
			}

			if _, ok := result.Content[1].(*mcp.ImageContent); !ok {
				t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
			}
		})
	}
}

func TestConvertCallToolRejectsSameFormat(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testImagePNG(t))
	}))
	t.Cleanup(images.Close)

	store := &fakeStore{}

	srv, err := New(Deps{Log: zapNop(), Publisher: publish.New(store, zapNop()), Resolver: newResolver(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "convert",
		Arguments: map[string]any{"image_url": images.URL + "/x.png", "format": "png"},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("CallTool() result = %+v, want a tool error for png to png", result.Content)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "convert:") || !strings.Contains(text.Text, "png") {
		t.Fatalf("content[0] = %#v, want an error naming convert and png", result.Content[0])
	}

	if len(store.uploads) != 0 {
		t.Errorf("uploads = %d, want a refused conversion to store nothing", len(store.uploads))
	}
}

func TestConvertCallToolErrors(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not an image"))
	}))
	t.Cleanup(images.Close)

	tests := []struct {
		name string
		in   convertInput
	}{
		{name: "invalid format", in: convertInput{ImageURL: images.URL + "/x.png", Format: "gif"}},
		{name: "undecodable image", in: convertInput{ImageURL: images.URL + "/x.png", Format: "png"}},
		{name: "fetch failure", in: convertInput{ImageURL: "http://127.0.0.1:1/x.png", Format: "png"}},
	}

	h := &handlers{log: zapNop(), resolver: newResolver(t), publisher: newTestPublisher(t)}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := h.convert(context.Background(), nil, tt.in); err == nil || !strings.HasPrefix(err.Error(), "convert:") {
				t.Fatalf("error = %v, want a convert: prefix", err)
			}
		})
	}
}
