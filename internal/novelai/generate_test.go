package novelai

import (
	"context"
	"image"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/format"
)

func initImagePNG(t *testing.T, width, height int) []byte {
	t.Helper()

	encoded, err := format.Encode(image.NewNRGBA(image.Rect(0, 0, width, height)), format.PNG)
	if err != nil {
		t.Fatalf("encode init image: %v", err)
	}

	return encoded
}

func newGenerateFrame(t *testing.T) []byte {
	t.Helper()

	return encodeFrame(t, map[string]any{"event_type": "final", "image": []byte("png")})
}

func TestGenerateTxt2ImgWithoutInitImage(t *testing.T) {
	server, captured := newGenerateServer(t, newGenerateFrame(t))

	client := New(server.URL, "sk-test")

	if _, err := client.Generate(context.Background(), GenerateRequest{
		Model:  "nai-diffusion-5-full",
		Prompt: "a cat",
	}); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	body := decodeJSON(t, captured.body)

	if body["action"] != "generate" {
		t.Errorf("action = %v, want generate", body["action"])
	}

	params := nestedMap(t, body, "parameters")

	if got := asFloat(t, params, "width"); got != defaultDimension {
		t.Errorf("width = %v, want the %d default", got, defaultDimension)
	}

	if got := asFloat(t, params, "height"); got != defaultDimension {
		t.Errorf("height = %v, want the %d default", got, defaultDimension)
	}

	for _, key := range []string{"image", "strength", "noise"} {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present, want it dropped without an init image", key)
		}
	}
}

func TestGenerateImg2ImgWithInitImage(t *testing.T) {
	server, captured := newGenerateServer(t, newGenerateFrame(t))

	client := New(server.URL, "sk-test")
	client.randomSeed = func() uint32 { return 99 }

	if _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     "nai-diffusion-5-full",
		Prompt:    "a cat",
		InitImage: initImagePNG(t, 1024, 768),
		Noise:     0.1,
	}); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	body := decodeJSON(t, captured.body)

	if body["action"] != "img2img" {
		t.Errorf("action = %v, want img2img", body["action"])
	}

	params := nestedMap(t, body, "parameters")

	if got := asFloat(t, params, "strength"); got != defaultStrength {
		t.Errorf("strength = %v, want the %v default", got, defaultStrength)
	}

	if got := asFloat(t, params, "noise"); got != 0.1 {
		t.Errorf("noise = %v, want 0.1", got)
	}

	if params["image"] == "" || params["image"] == nil {
		t.Error("image is missing, want the init image base64 encoded")
	}

	if got := asFloat(t, params, "width"); got != 1024 {
		t.Errorf("width = %v, want the init image's 1024", got)
	}
}

func TestGenerateImg2ImgKeepsRequestedStrength(t *testing.T) {
	server, captured := newGenerateServer(t, newGenerateFrame(t))

	client := New(server.URL, "sk-test")

	if _, err := client.Generate(context.Background(), GenerateRequest{
		Model:     "nai-diffusion-5-full",
		InitImage: initImagePNG(t, 512, 512),
		Strength:  0.4,
	}); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	params := nestedMap(t, decodeJSON(t, captured.body), "parameters")

	if got := asFloat(t, params, "strength"); got != 0.4 {
		t.Errorf("strength = %v, want 0.4", got)
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
			name:      "a multiple of 64 is kept",
			initWidth: 320, initHeight: 640, wantWidth: 320, wantHeight: 640,
		},
		{
			name:      "the init image's grid size is rounded up to a multiple of 64",
			initWidth: 1003, initHeight: 667, wantWidth: 1024, wantHeight: 704,
		},
		{
			name:      "both set are rounded up",
			req:       GenerateRequest{Width: 833, Height: 1217},
			initWidth: 512, initHeight: 512, wantWidth: 896, wantHeight: 1280,
		},
		{
			name:      "width alone keeps the aspect ratio",
			req:       GenerateRequest{Width: 1536},
			initWidth: 768, initHeight: 512, wantWidth: 1536, wantHeight: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, captured := newGenerateServer(t, newGenerateFrame(t))

			tt.req.Model = "nai-diffusion-5-full"
			tt.req.InitImage = initImagePNG(t, tt.initWidth, tt.initHeight)

			if _, err := New(server.URL, "sk-test").Generate(context.Background(), tt.req); err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			params := nestedMap(t, decodeJSON(t, captured.body), "parameters")

			width, height := asFloat(t, params, "width"), asFloat(t, params, "height")
			if width != tt.wantWidth || height != tt.wantHeight {
				t.Errorf("dimensions = %vx%v, want %vx%v", width, height, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestGenerateWithUnreadableInitImage(t *testing.T) {
	client := New("http://example.com", "sk-test")

	_, err := client.Generate(context.Background(), GenerateRequest{
		Model:     "nai-diffusion-5-full",
		InitImage: []byte("not an image"),
	})
	if err == nil || !strings.Contains(err.Error(), "read init image size") {
		t.Fatalf("Generate() error = %v, want it to name the init image size", err)
	}
}
