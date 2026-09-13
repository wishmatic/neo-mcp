package sdwebui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImg2ImgPayload(t *testing.T) {
	var (
		gotPath string
		gotBody []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images": []}`))
	}))
	defer server.Close()

	c := New(server.URL, false)

	_, err := c.Img2Img(context.Background(), Img2ImgRequest{
		Checkpoint:        "model",
		InitImageData:     []byte("png"),
		Prompt:            "test",
		DenoisingStrength: 0.6,
	})
	if err != nil {
		t.Fatalf("Img2Img() error: %v", err)
	}

	if gotPath != "/sdapi/v1/img2img" {
		t.Errorf("path = %q, want /sdapi/v1/img2img", gotPath)
	}

	var payload struct {
		InitImages        []string `json:"init_images"`
		DenoisingStrength float64  `json:"denoising_strength"`
		OverrideSettings  struct {
			Checkpoint string `json:"sd_model_checkpoint"`
		} `json:"override_settings"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if len(payload.InitImages) != 1 || !strings.HasPrefix(payload.InitImages[0], "data:image/png;base64,") {
		t.Errorf("init_images = %v, want one base64 data URI", payload.InitImages)
	}
	if payload.DenoisingStrength != 0.6 {
		t.Errorf("denoising_strength = %v, want 0.6", payload.DenoisingStrength)
	}
	if payload.OverrideSettings.Checkpoint != "model.safetensors" {
		t.Errorf("sd_model_checkpoint = %q, want model.safetensors", payload.OverrideSettings.Checkpoint)
	}
}

func TestFetchImageNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := New(server.URL, false)

	if _, err := c.FetchImage(context.Background(), server.URL+"/missing.png"); err == nil {
		t.Fatal("FetchImage() expected error, got nil")
	}
}
