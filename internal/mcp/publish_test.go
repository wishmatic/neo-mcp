package mcp

import (
	"bytes"
	"context"
	"errors"
	"image"
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

			result, out, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, format, returnURL)
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

	if _, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, true, imgfmt.PNG, returnURL); err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	if len(store.uploads) != 1 || !store.uploads[0].nsfw {
		t.Fatalf("uploads = %+v, want one NSFW upload", store.uploads)
	}
}

func TestPublishImagesUploadFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: publish.New(&fakeStore{err: errors.New("boom")}, zap.NewNop())}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, imgfmt.PNG, returnURL)
	if err == nil {
		t.Fatal("publishImages() error = nil, want the upload failure")
	}
}

func TestPublishImagesConvertFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{[]byte("not an image")}, false, imgfmt.WebP, returnURL)
	if err == nil || !strings.HasPrefix(err.Error(), "txt2img:") {
		t.Fatalf("error = %v, want a txt2img: prefix", err)
	}
}

func TestPublishImagesInline(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

	result, out, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, imgfmt.PNG, returnImage)
	if err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	if out.Count != 1 || len(out.URLs) != 1 {
		t.Fatalf("output = %+v, want one URL", out)
	}

	if len(result.Content) != 2 {
		t.Fatalf("content = %d, want a caption and one image", len(result.Content))
	}

	caption, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(caption.Text, out.URLs[0]) {
		t.Fatalf("content[0] = %#v, want a caption naming the stored URL", result.Content[0])
	}

	img, ok := result.Content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
	}

	if img.MIMEType != "image/webp" {
		t.Errorf("mime type = %q, want image/webp", img.MIMEType)
	}

	if len(img.Data) == 0 {
		t.Fatal("image data is empty")
	}

	if _, format, err := image.Decode(bytes.NewReader(img.Data)); err != nil || format != "webp" {
		t.Errorf("decode inline image = %q, %v, want webp", format, err)
	}
}

func TestInlineContentFailsSoft(t *testing.T) {
	h := &handlers{log: zapNop()}

	content := h.imageContent([][]byte{[]byte("not an image")}, []string{"https://cdn.example.com/i/1.png"}, returnImage)
	if len(content) != 2 {
		t.Fatalf("content = %d, want a caption and a failure note", len(content))
	}

	if _, ok := content[1].(*mcp.ImageContent); ok {
		t.Fatal("content[1] is an image block, want a text failure note")
	}

	note, ok := content[1].(*mcp.TextContent)
	if !ok || !strings.Contains(note.Text, "could not be attached inline") {
		t.Fatalf("content[1] = %#v, want a failure note", content[1])
	}
}
