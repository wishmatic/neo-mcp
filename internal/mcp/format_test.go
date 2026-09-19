package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

const testImageSize = 4

func testImagePNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, testImageSize, testImageSize))

	for y := 0; y < testImageSize; y++ {
		for x := 0; x < testImageSize; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 180, G: 40, B: 10, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func testImageJPEG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, testImageSize, testImageSize))

	for y := 0; y < testImageSize; y++ {
		for x := 0; x < testImageSize; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 160, B: 90, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	return buf.Bytes()
}

func TestOutputFormatResolver(t *testing.T) {
	configured := &handlers{log: zapNop(), defaultFormat: imgfmt.JXL}

	tests := []struct {
		name    string
		h       *handlers
		flag    string
		want    imgfmt.Format
		wantErr bool
	}{
		{name: "configured default", h: configured, want: imgfmt.JXL},
		{name: "unset default falls back to webp", h: &handlers{log: zapNop()}, want: imgfmt.Default},
		{name: "flag overrides the default", h: configured, flag: "png", want: imgfmt.PNG},
		{name: "flag alias", h: configured, flag: "jpg", want: imgfmt.JPEG},
		{name: "invalid flag", h: configured, flag: "gif", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.h.outputFormat(tt.flag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("outputFormat(%q) error = nil, want an error", tt.flag)
				}

				return
			}

			if err != nil {
				t.Fatalf("outputFormat(%q) error: %v", tt.flag, err)
			}

			if got != tt.want {
				t.Errorf("outputFormat(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestBuildHandlersDefaultsFormat(t *testing.T) {
	h := buildHandlers(Deps{Log: zapNop()})

	if h.defaultFormat != imgfmt.Default {
		t.Errorf("defaultFormat = %q, want %q", h.defaultFormat, imgfmt.Default)
	}
}

func TestTxt2ImgRejectsInvalidFormatBeforeGenerating(t *testing.T) {
	h := &handlers{log: zapNop()}

	_, _, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInput{Model: "m.safetensors", Format: "gif"},
	})
	if err == nil || !strings.HasPrefix(err.Error(), "txt2img:") {
		t.Fatalf("error = %v, want a txt2img: prefix", err)
	}
}

func TestBgkillCallToolFormatsOutput(t *testing.T) {
	forge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"output_image": base64.StdEncoding.EncodeToString(testImagePNG(t)),
		})
	}))
	t.Cleanup(forge.Close)

	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testImagePNG(t))
	}))
	t.Cleanup(images.Close)

	resolver, err := resolve.New(nil, "")
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	srv, err := New(Deps{
		Log:          zapNop(),
		Bgkill:       bgkill.New(sdwebui.New(forge.URL, false)),
		Publisher:    newTestPublisher(t),
		Resolver:     resolver,
		OutputFormat: imgfmt.Default,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "bgkill",
		Arguments: map[string]any{
			"model_name": bgkill.Models[0],
			"image_url":  images.URL + "/x.png",
			"format":     "jpeg",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.HasSuffix(content.Text, ".jpg") {
		t.Fatalf("content = %#v, want an uploaded .jpg URL", result.Content[0])
	}
}
