package forge

import (
	"context"
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/format"
	"go.uber.org/zap"
)

type recordedRequest struct {
	path string
	body []byte
}

func newRecordingForge(t *testing.T) (*Client, *recordedRequest) {
	t.Helper()

	recorded := &recordedRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded.path = r.URL.Path
		recorded.body, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images": ["cG5n"]}`))
	}))

	t.Cleanup(server.Close)

	return New(server.URL, zap.NewNop()), recorded
}

func initImagePNG(t *testing.T, width, height int) []byte {
	t.Helper()

	encoded, err := format.Encode(image.NewNRGBA(image.Rect(0, 0, width, height)), format.PNG)
	if err != nil {
		t.Fatalf("encode init image: %v", err)
	}

	return encoded
}

func payloadOf(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	return payload
}

func TestGenerateTxt2ImgWithoutInitImage(t *testing.T) {
	client, recorded := newRecordingForge(t)

	if _, err := client.Generate(context.Background(), GenerateRequest{Prompt: "a cat"}); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if recorded.path != "/sdapi/v1/txt2img" {
		t.Errorf("path = %q, want /sdapi/v1/txt2img", recorded.path)
	}

	payload := payloadOf(t, recorded.body)

	if payload["width"] != float64(defaultDimension) || payload["height"] != float64(defaultDimension) {
		t.Errorf("dimensions = %vx%v, want the %d default", payload["width"], payload["height"], defaultDimension)
	}

	if _, ok := payload["init_images"]; ok {
		t.Error("init_images is present, want none without an init image")
	}
}

func TestGenerateTxt2ImgDefaultsOnlyTheMissingDimension(t *testing.T) {
	client, recorded := newRecordingForge(t)

	if _, err := client.Generate(context.Background(), GenerateRequest{Prompt: "a cat", Width: 768}); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	payload := payloadOf(t, recorded.body)

	if payload["width"] != float64(768) || payload["height"] != float64(defaultDimension) {
		t.Errorf("dimensions = %vx%v, want 768x%d", payload["width"], payload["height"], defaultDimension)
	}
}

func TestGenerateImg2ImgWithInitImage(t *testing.T) {
	client, recorded := newRecordingForge(t)

	initImage := initImagePNG(t, 1024, 768)

	if _, err := client.Generate(context.Background(), GenerateRequest{Prompt: "a cat", InitImage: initImage}); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if recorded.path != "/sdapi/v1/img2img" {
		t.Errorf("path = %q, want /sdapi/v1/img2img", recorded.path)
	}

	payload := payloadOf(t, recorded.body)

	if payload["denoising_strength"] != defaultDenoisingStrength {
		t.Errorf("denoising_strength = %v, want %v", payload["denoising_strength"], defaultDenoisingStrength)
	}

	images, ok := payload["init_images"].([]any)
	if !ok || len(images) != 1 || images[0] == "" {
		t.Fatalf("init_images = %v, want the init image base64 encoded", payload["init_images"])
	}

	if payload["width"] != float64(1024) || payload["height"] != float64(768) {
		t.Errorf("dimensions = %vx%v, want the init image's 1024x768", payload["width"], payload["height"])
	}
}

func TestGenerateImg2ImgKeepsRequestedStrength(t *testing.T) {
	client, recorded := newRecordingForge(t)

	req := GenerateRequest{Prompt: "a cat", InitImage: initImagePNG(t, 512, 512), DenoisingStrength: 0.4}

	if _, err := client.Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if got := payloadOf(t, recorded.body)["denoising_strength"]; got != 0.4 {
		t.Errorf("denoising_strength = %v, want 0.4", got)
	}
}

func TestGenerateSizesImg2ImgFromInitImage(t *testing.T) {
	tests := []struct {
		name       string
		req        GenerateRequest
		initWidth  int
		initHeight int
		wantWidth  float64
		wantHeight float64
	}{
		{
			name:      "neither set takes the init image's size",
			initWidth: 768, initHeight: 512, wantWidth: 768, wantHeight: 512,
		},
		{
			name:      "neither set snaps to the grid",
			initWidth: 1003, initHeight: 667, wantWidth: 1000, wantHeight: 664,
		},
		{
			name:      "both set are kept",
			req:       GenerateRequest{Width: 512, Height: 1024},
			initWidth: 768, initHeight: 512, wantWidth: 512, wantHeight: 1024,
		},
		{
			name:      "width alone keeps the aspect ratio",
			req:       GenerateRequest{Width: 1536},
			initWidth: 768, initHeight: 512, wantWidth: 1536, wantHeight: 1024,
		},
		{
			name:      "height alone keeps the aspect ratio and snaps",
			req:       GenerateRequest{Height: 256},
			initWidth: 1003, initHeight: 667, wantWidth: 384, wantHeight: 256,
		},
		{
			name:      "a tiny image still gets the grid minimum",
			req:       GenerateRequest{Height: 1},
			initWidth: 1, initHeight: 4096, wantWidth: 8, wantHeight: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, recorded := newRecordingForge(t)

			tt.req.Prompt = "a cat"
			tt.req.InitImage = initImagePNG(t, tt.initWidth, tt.initHeight)

			if _, err := client.Generate(context.Background(), tt.req); err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			payload := payloadOf(t, recorded.body)

			if payload["width"] != tt.wantWidth || payload["height"] != tt.wantHeight {
				t.Errorf("dimensions = %vx%v, want %vx%v",
					payload["width"], payload["height"], tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestGenerateWithUnreadableInitImage(t *testing.T) {
	client, _ := newRecordingForge(t)

	_, err := client.Generate(context.Background(), GenerateRequest{
		Prompt:    "a cat",
		InitImage: []byte("not an image"),
	})
	if err == nil || !strings.Contains(err.Error(), "read init image size") {
		t.Fatalf("Generate() error = %v, want it to name the init image size", err)
	}
}
