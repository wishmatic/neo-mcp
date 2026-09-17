package imagegen

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProviderOf(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{model: "nai-diffusion-5-full", want: "novelai"},
		{model: " nai-diffusion-4-5-full ", want: "novelai"},
		{model: "sd_xl_base_1.0.safetensors", want: "forge"},
		{model: "", want: "forge"},
	}

	for _, tt := range tests {
		if got := ProviderOf(tt.model); got != tt.want {
			t.Errorf("ProviderOf(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestTxt2ImgRoutesNovelAIModel(t *testing.T) {
	forgeLog := &requestLog{}
	novelaiLog := &requestLog{}

	g := New(newForgeBackend(t, forgeLog), newNovelAIBackend(t, novelaiLog))

	images, err := g.Txt2Img(context.Background(), txt2ImgRequestFor("nai-diffusion-5-full"))
	if err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	if len(images) != 1 || string(images[0]) != "novelai-png" {
		t.Errorf("images = %v, want novelai-png", images)
	}

	if paths, _ := forgeLog.snapshot(); len(paths) != 0 {
		t.Errorf("forge received %v, want no requests", paths)
	}

	paths, bodies := novelaiLog.snapshot()
	if len(paths) != 1 || paths[0] != "/ai/generate-image-stream" {
		t.Fatalf("novelai received %v, want one stream request", paths)
	}

	if body := decodeJSONBody(t, bodies[0]); body["model"] != "nai-diffusion-5-full" {
		t.Errorf("model = %v, want nai-diffusion-5-full", body["model"])
	}
}

func TestTxt2ImgRoutesForgeModel(t *testing.T) {
	forgeLog := &requestLog{}
	novelaiLog := &requestLog{}

	g := New(newForgeBackend(t, forgeLog), newNovelAIBackend(t, novelaiLog))

	images, err := g.Txt2Img(context.Background(), txt2ImgRequestFor("sd_xl_base_1.0.safetensors"))
	if err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	if len(images) != 1 || string(images[0]) != "forge-png" {
		t.Errorf("images = %v, want forge-png", images)
	}

	if paths, _ := novelaiLog.snapshot(); len(paths) != 0 {
		t.Errorf("novelai received %v, want no requests", paths)
	}

	paths, _ := forgeLog.snapshot()
	if len(paths) != 1 || paths[0] != "/sdapi/v1/txt2img" {
		t.Fatalf("forge received %v, want one txt2img request", paths)
	}
}

func TestTxt2ImgNovelAIWithoutClient(t *testing.T) {
	g := New(newForgeBackend(t, &requestLog{}), nil)

	_, err := g.Txt2Img(context.Background(), txt2ImgRequestFor("nai-diffusion-5-full"))
	if err == nil {
		t.Fatal("Txt2Img() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "NOVELAI_API_KEY") {
		t.Errorf("error = %q, want it to name NOVELAI_API_KEY", err.Error())
	}
}

func TestTxt2ImgForgeWithoutClient(t *testing.T) {
	g := New(nil, newNovelAIBackend(t, &requestLog{}))

	_, err := g.Txt2Img(context.Background(), txt2ImgRequestFor("sd_xl_base_1.0.safetensors"))
	if !errors.Is(err, ErrForgeNotConfigured) {
		t.Errorf("error = %v, want ErrForgeNotConfigured", err)
	}
}

func TestImg2ImgNovelAIWithoutClient(t *testing.T) {
	g := New(newForgeBackend(t, &requestLog{}), nil)

	_, err := g.Img2Img(context.Background(), Img2ImgRequest{
		Params:    Params{Model: "nai-diffusion-5-full", Prompt: "a cat", Width: 512, Height: 512},
		InitImage: []byte("\x89PNG\r\n\x1a\n"),
	})
	if err == nil {
		t.Fatal("Img2Img() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "NOVELAI_API_KEY") {
		t.Errorf("error = %q, want it to name NOVELAI_API_KEY", err.Error())
	}
}

func TestImg2ImgForgeWithoutClient(t *testing.T) {
	g := New(nil, newNovelAIBackend(t, &requestLog{}))

	_, err := g.Img2Img(context.Background(), Img2ImgRequest{
		Params:    Params{Model: "sd_xl_base_1.0.safetensors", Prompt: "a cat", Width: 512, Height: 512},
		InitImage: []byte("\x89PNG\r\n\x1a\n"),
	})
	if !errors.Is(err, ErrForgeNotConfigured) {
		t.Errorf("error = %v, want ErrForgeNotConfigured", err)
	}
}
