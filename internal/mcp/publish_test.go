package mcp

import (
	"bytes"
	"context"
	"image"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/publish"
)

func TestPublishImagesWithUploader(t *testing.T) {
	for _, public := range []bool{false, true} {
		for _, format := range []imgfmt.Format{imgfmt.PNG, imgfmt.JPEG, imgfmt.JXL, imgfmt.WebP} {
			t.Run(format.String(), func(t *testing.T) {
				captured := &[]uploadCapture{}
				h := &handlers{log: zapNop(), publisher: publish.New(newPublicizeUploader(t, captured), nil, zapNop())}

				images := [][]byte{testImagePNG(t)}
				result, out, err := h.publishImages(context.Background(), "txt2img", images, public, format)
				if err != nil {
					t.Fatalf("publishImages(public=%v, format=%s) error: %v", public, format, err)
				}

				if out.Count != 1 || len(out.URLs) != 1 {
					t.Fatalf("output = %+v, want one URL", out)
				}

				if len(*captured) != 1 || (*captured)[0].contentType != format.MediaType() {
					t.Fatalf("uploads = %+v, want content type %s", *captured, format.MediaType())
				}

				if len(result.Content) != 1 {
					t.Fatalf("content = %d, want 1", len(result.Content))
				}

				text, ok := result.Content[0].(*mcp.TextContent)
				if !ok || text.Text != out.URLs[0] {
					t.Fatalf("content = %#v, want the uploaded URL", result.Content[0])
				}
			})
		}
	}
}

func TestPublishImagesWithoutUploader(t *testing.T) {
	for _, format := range []imgfmt.Format{imgfmt.PNG, imgfmt.JPEG, imgfmt.JXL, imgfmt.WebP} {
		t.Run(format.String(), func(t *testing.T) {
			h := &handlers{log: zapNop(), publisher: publish.New(nil, nil, zapNop())}

			images := [][]byte{testImagePNG(t)}
			result, out, err := h.publishImages(context.Background(), "txt2img", images, false, format)
			if err != nil {
				t.Fatalf("publishImages(format=%s) error: %v", format, err)
			}

			if out.Count != 1 || len(out.URLs) != 0 {
				t.Fatalf("output = %+v, want one image and no URLs", out)
			}

			if len(result.Content) != 1 {
				t.Fatalf("content = %d, want 1", len(result.Content))
			}

			content, ok := result.Content[0].(*mcp.ImageContent)
			if !ok || content.MIMEType != format.MediaType() {
				t.Fatalf("content = %#v, want inline %s", result.Content[0], format.MediaType())
			}

			img, decoded, err := image.Decode(bytes.NewReader(content.Data))
			if err != nil {
				t.Fatalf("decode returned content: %v", err)
			}

			if decoded != format.String() {
				t.Errorf("decoded format = %q, want %q", decoded, format)
			}

			if bounds := img.Bounds(); bounds.Dx() != 4 || bounds.Dy() != 4 {
				t.Errorf("bounds = %v, want 4x4", bounds)
			}
		})
	}
}

func TestPublishImagesConvertFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: publish.New(nil, nil, zapNop())}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{[]byte("not an image")}, false, imgfmt.WebP)
	if err == nil || !strings.HasPrefix(err.Error(), "txt2img:") {
		t.Fatalf("error = %v, want a txt2img: prefix", err)
	}
}
