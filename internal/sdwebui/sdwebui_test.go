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

func TestTxt2ImgOverrideSettings(t *testing.T) {
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images": []}`))
	}))
	defer server.Close()

	c := New(server.URL, false)

	_, err := c.Txt2Img(context.Background(), Txt2ImgRequest{
		Checkpoint:             "model",
		ForgePreset:            "flux",
		ForgeAdditionalModules: []string{"vae", "t5xxl.ckpt"},
		Prompt:                 "test",
	})
	if err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	var payload struct {
		OverrideSettings struct {
			Checkpoint             string   `json:"sd_model_checkpoint"`
			ForgePreset            string   `json:"forge_preset"`
			ForgeAdditionalModules []string `json:"forge_additional_modules"`
		} `json:"override_settings"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.OverrideSettings.Checkpoint != "model.safetensors" {
		t.Errorf("sd_model_checkpoint = %q, want model.safetensors", payload.OverrideSettings.Checkpoint)
	}
	if payload.OverrideSettings.ForgePreset != "flux" {
		t.Errorf("forge_preset = %q, want flux", payload.OverrideSettings.ForgePreset)
	}
	if len(payload.OverrideSettings.ForgeAdditionalModules) != 2 {
		t.Fatalf("forge_additional_modules = %v, want 2 entries", payload.OverrideSettings.ForgeAdditionalModules)
	}
	if payload.OverrideSettings.ForgeAdditionalModules[0] != "vae.safetensors" {
		t.Errorf("forge_additional_modules[0] = %q, want vae.safetensors", payload.OverrideSettings.ForgeAdditionalModules[0])
	}
	if payload.OverrideSettings.ForgeAdditionalModules[1] != "t5xxl.ckpt" {
		t.Errorf("forge_additional_modules[1] = %q, want t5xxl.ckpt (already has extension)", payload.OverrideSettings.ForgeAdditionalModules[1])
	}
}

func TestEnsureSafetensors(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"qwen_image_vae", "qwen_image_vae.safetensors"},
		{"animiji_s1_txt.safetensors", "animiji_s1_txt.safetensors"},
		{"model.ckpt", "model.ckpt"},
		{"model.pt", "model.pt"},
		{"model.bin", "model.bin"},
		{"model.gguf", "model.gguf"},
		{"MODEL", "MODEL.safetensors"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := ensureSafetensors(tt.in); got != tt.want {
			t.Errorf("ensureSafetensors(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTxt2ImgNoOverrideSettings(t *testing.T) {
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images": []}`))
	}))
	defer server.Close()

	c := New(server.URL, false)

	_, err := c.Txt2Img(context.Background(), Txt2ImgRequest{Prompt: "test"})
	if err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	if strings.Contains(string(gotBody), "override_settings") {
		t.Errorf("payload should not contain override_settings when empty, got: %s", gotBody)
	}
}
