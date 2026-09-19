package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"go.uber.org/zap"
)

func TestPublishImagesUploads(t *testing.T) {
	for _, format := range []imgfmt.Format{imgfmt.PNG, imgfmt.JPEG, imgfmt.JXL, imgfmt.WebP} {
		t.Run(format.String(), func(t *testing.T) {
			h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

			result, out, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, format)
			if err != nil {
				t.Fatalf("publishImages(format=%s) error: %v", format, err)
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
		})
	}
}

func TestPublishImagesNSFW(t *testing.T) {
	store := &fakeStore{}
	h := &handlers{log: zapNop(), publisher: publish.New(store, zap.NewNop())}

	if _, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, true, imgfmt.PNG); err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	if len(store.uploads) != 1 || !store.uploads[0].nsfw {
		t.Fatalf("uploads = %+v, want one NSFW upload", store.uploads)
	}
}

func TestPublishImagesUploadFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: publish.New(&fakeStore{err: errors.New("boom")}, zap.NewNop())}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, imgfmt.PNG)
	if err == nil {
		t.Fatal("publishImages() error = nil, want the upload failure")
	}
}

func TestPublishImagesConvertFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{[]byte("not an image")}, false, imgfmt.WebP)
	if err == nil || !strings.HasPrefix(err.Error(), "txt2img:") {
		t.Fatalf("error = %v, want a txt2img: prefix", err)
	}
}
