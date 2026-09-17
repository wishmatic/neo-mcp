package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/publish"
)

func TestPublishImagesWithUploader(t *testing.T) {
	for _, public := range []bool{false, true} {
		uploader := newPublicizeUploader(t, &[]uploadCapture{})
		h := &handlers{log: zapNop(), publisher: publish.New(uploader, nil, zapNop())}

		result, out, err := h.publishImages(context.Background(), "txt2img", [][]byte{[]byte("pretend-png-bytes")}, public)
		if err != nil {
			t.Fatalf("publishImages(public=%v) error: %v", public, err)
		}

		if out.Count != 1 || len(out.URLs) != 1 {
			t.Fatalf("output = %+v, want one URL", out)
		}

		if len(result.Content) != 1 {
			t.Fatalf("content = %d, want 1", len(result.Content))
		}

		text, ok := result.Content[0].(*mcp.TextContent)
		if !ok || text.Text != out.URLs[0] {
			t.Fatalf("content = %#v, want the uploaded URL", result.Content[0])
		}
	}
}

func TestPublishImagesWithoutUploader(t *testing.T) {
	for _, public := range []bool{false, true} {
		h := &handlers{log: zapNop(), publisher: publish.New(nil, nil, zapNop())}

		result, out, err := h.publishImages(context.Background(), "txt2img", [][]byte{[]byte("pretend-png-bytes")}, public)
		if err != nil {
			t.Fatalf("publishImages(public=%v) error: %v", public, err)
		}

		if out.Count != 1 || len(out.URLs) != 0 {
			t.Fatalf("output = %+v, want one image and no URLs", out)
		}

		if len(result.Content) != 1 {
			t.Fatalf("content = %d, want 1", len(result.Content))
		}

		image, ok := result.Content[0].(*mcp.ImageContent)
		if !ok || image.MIMEType != "image/png" {
			t.Fatalf("content = %#v, want inline image/png", result.Content[0])
		}
	}
}
