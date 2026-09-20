package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"go.uber.org/zap"
)

func TestPublishImagesReturnsURLAndImage(t *testing.T) {
	for _, format := range []imgfmt.Format{imgfmt.PNG, imgfmt.JPEG, imgfmt.JXL, imgfmt.WebP} {
		t.Run(format.String(), func(t *testing.T) {
			h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

			result, out, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, format, false)
			if err != nil {
				t.Fatalf("publishImages(format=%s) error: %v", format, err)
			}

			if out.Count != 1 || len(out.URLs) != 1 {
				t.Fatalf("output = %+v, want one URL", out)
			}

			if len(result.Content) != 2 {
				t.Fatalf("content = %d, want a URL and one image", len(result.Content))
			}

			text, ok := result.Content[0].(*mcp.TextContent)
			if !ok || text.Text != out.URLs[0] {
				t.Fatalf("content[0] = %#v, want the uploaded URL as text", result.Content[0])
			}

			img, ok := result.Content[1].(*mcp.ImageContent)
			if !ok {
				t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
			}

			if img.MIMEType != "image/webp" {
				t.Errorf("mime type = %q, want image/webp", img.MIMEType)
			}

			if _, decoded, err := image.Decode(bytes.NewReader(img.Data)); err != nil || decoded != "webp" {
				t.Errorf("decode inline image = %q, %v, want webp", decoded, err)
			}
		})
	}
}

func TestImageContentWireShape(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

	result, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, imgfmt.PNG, true)
	if err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	raw, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode content: %v", err)
	}

	if decoded[0]["type"] != "text" || decoded[0]["text"] == "" {
		t.Errorf("text block = %v, want {type: text, text: ...}", decoded[0])
	}

	if decoded[1]["type"] != "image" || decoded[1]["mimeType"] != "image/webp" || decoded[1]["data"] == "" {
		t.Errorf("image block = %v, want {type: image, mimeType: image/webp, data: ...}", decoded[1])
	}

	annotations, ok := decoded[1]["annotations"].(map[string]any)
	if !ok {
		t.Fatalf("image block annotations = %v, want an object", decoded[1]["annotations"])
	}

	audience, ok := annotations["audience"].([]any)
	if !ok || !slices.Equal(audience, []any{"assistant", "user"}) {
		t.Errorf("audience = %v, want [assistant user]", annotations["audience"])
	}
}

func TestPublishImagesNSFW(t *testing.T) {
	store := &fakeStore{}
	h := &handlers{log: zapNop(), publisher: publish.New(store, zap.NewNop())}

	if _, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, true, imgfmt.PNG, false); err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	if len(store.uploads) != 1 || !store.uploads[0].nsfw {
		t.Fatalf("uploads = %+v, want one NSFW upload", store.uploads)
	}
}

func TestPublishImagesUploadFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: publish.New(&fakeStore{err: errors.New("boom")}, zap.NewNop())}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{testImagePNG(t)}, false, imgfmt.PNG, false)
	if err == nil {
		t.Fatal("publishImages() error = nil, want the upload failure")
	}
}

func TestPublishImagesConvertFailure(t *testing.T) {
	h := &handlers{log: zapNop(), publisher: newTestPublisher(t)}

	_, _, err := h.publishImages(context.Background(), "txt2img", [][]byte{[]byte("not an image")}, false, imgfmt.WebP, false)
	if err == nil || !strings.HasPrefix(err.Error(), "txt2img:") {
		t.Fatalf("error = %v, want a txt2img: prefix", err)
	}
}
