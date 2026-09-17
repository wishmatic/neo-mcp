package imagegen

import (
	"context"
	"encoding/base64"
	"testing"
)

func TestForgeTxt2ImgRequestMapsFields(t *testing.T) {
	req := Txt2ImgRequest{
		Params: Params{
			Model:             "m.safetensors",
			ForgePreset:       "krea",
			VAEAndTextModels:  []string{"vae.safetensors"},
			Prompt:            "p",
			NegativePrompt:    "n",
			Sampler:           "Euler",
			Scheduler:         "Karras",
			Steps:             30,
			Width:             768,
			Height:            512,
			CFGScale:          6.5,
			Seed:              42,
			EnableHR:          true,
			HRScale:           2,
			HRUpscaler:        "4x",
			HRSecondPassSteps: 12,
			HRCFGScale:        5,
		},
		HRDenoisingStrength: 0.4,
	}

	got := forgeTxt2ImgRequest(req)

	if got.Checkpoint != "m.safetensors" || got.ForgePreset != "krea" || got.ForgeAdditionalModules[0] != "vae.safetensors" {
		t.Errorf("forge-only fields = %+v, want them copied", got)
	}

	if got.SamplerName != "Euler" || got.Scheduler != "Karras" || got.Steps != 30 {
		t.Errorf("sampling fields = %+v, want them copied", got)
	}

	if got.Width != 768 || got.Height != 512 || got.CFGScale != 6.5 || got.Seed != 42 {
		t.Errorf("dimensions and seed = %+v, want them copied", got)
	}

	if !got.EnableHR || got.HRScale != 2 || got.HRUpscaler != "4x" || got.HRSecondPassSteps != 12 ||
		got.HRCFGScale != 5 || got.DenoisingStrength != 0.4 {
		t.Errorf("HR fields = %+v, want them copied with the hr denoising strength", got)
	}
}

func TestTxt2ImgForgeSamplerDefaults(t *testing.T) {
	forgeLog := &requestLog{}

	g := New(newForgeBackend(t, forgeLog), nil)

	if _, err := g.Txt2Img(context.Background(), txt2ImgRequestFor("sd_xl_base_1.0.safetensors")); err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	_, bodies := forgeLog.snapshot()
	body := decodeJSONBody(t, bodies[0])

	if body["sampler_name"] != DefaultSampler {
		t.Errorf("sampler_name = %v, want %s", body["sampler_name"], DefaultSampler)
	}

	if body["scheduler"] != DefaultScheduler {
		t.Errorf("scheduler = %v, want %s", body["scheduler"], DefaultScheduler)
	}
}

func TestTxt2ImgNovelAIIgnoresForgeOnlyFields(t *testing.T) {
	novelaiLog := &requestLog{}

	g := New(nil, newNovelAIBackend(t, novelaiLog))

	req := txt2ImgRequestFor("nai-diffusion-5-full")
	req.HRDenoisingStrength = 0.5
	req.ForgePreset = "krea"
	req.VAEAndTextModels = []string{"vae.safetensors"}
	req.Scheduler = "Automatic"
	req.EnableHR = true
	req.HRScale = 2
	req.HRUpscaler = "4x-AnimeSharp"
	req.HRSecondPassSteps = 10
	req.HRCFGScale = 5

	if _, err := g.Txt2Img(context.Background(), req); err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	_, bodies := novelaiLog.snapshot()
	body := decodeJSONBody(t, bodies[0])
	params := paramsOf(t, body)

	for _, key := range []string{"forge_preset", "vae_and_text_models", "scheduler", "enable_hr"} {
		if _, ok := body[key]; ok {
			t.Errorf("%s is present at the top level, want it dropped", key)
		}
	}

	dropped := []string{
		"forge_preset", "vae_and_text_models", "scheduler", "enable_hr",
		"hr_scale", "hr_upscaler", "hr_second_pass_steps", "hr_cfg",
		"denoising_strength", "strength",
	}

	for _, key := range dropped {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present in the NovelAI payload, want it dropped", key)
		}
	}
}

func TestTxt2ImgNovelAISampler(t *testing.T) {
	tests := []struct {
		name    string
		sampler string
		want    string
	}{
		{name: "empty uses the NovelAI default", sampler: "", want: "k_euler_ancestral"},
		{name: "known sampler passes through", sampler: "k_dpmpp_2m", want: "k_dpmpp_2m"},
		{name: "unknown sampler passes through", sampler: "some_future_sampler", want: "some_future_sampler"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			novelaiLog := &requestLog{}
			g := New(nil, newNovelAIBackend(t, novelaiLog))

			req := txt2ImgRequestFor("nai-diffusion-5-full")
			req.Sampler = tt.sampler

			if _, err := g.Txt2Img(context.Background(), req); err != nil {
				t.Fatalf("Txt2Img() error: %v", err)
			}

			_, bodies := novelaiLog.snapshot()
			params := paramsOf(t, decodeJSONBody(t, bodies[0]))

			if params["sampler"] != tt.want {
				t.Errorf("sampler = %v, want %s", params["sampler"], tt.want)
			}
		})
	}
}

func TestImg2ImgNovelAIMapsStrengthAndNoise(t *testing.T) {
	novelaiLog := &requestLog{}

	g := New(nil, newNovelAIBackend(t, novelaiLog))

	initImage := []byte("\x89PNG\r\n\x1a\n")

	req := Img2ImgRequest{
		Params:    Params{Model: "nai-diffusion-5-full", Prompt: "a cat", Steps: 20, Width: 512, Height: 512, CFGScale: 7, Seed: -1},
		InitImage: initImage,
		Strength:  0.6,
		Noise:     0.1,
	}
	req.Scheduler = "Automatic"
	req.EnableHR = true

	if _, err := g.Img2Img(context.Background(), req); err != nil {
		t.Fatalf("Img2Img() error: %v", err)
	}

	_, bodies := novelaiLog.snapshot()
	body := decodeJSONBody(t, bodies[0])

	if body["action"] != "img2img" {
		t.Errorf("action = %v, want img2img", body["action"])
	}

	params := paramsOf(t, body)

	if got := numberField(t, params, "strength"); got != 0.6 {
		t.Errorf("strength = %v, want 0.6", got)
	}

	if got := numberField(t, params, "noise"); got != 0.1 {
		t.Errorf("noise = %v, want 0.1", got)
	}

	if params["image"] != base64.StdEncoding.EncodeToString(initImage) {
		t.Errorf("image = %v, want the raw base64 init image", params["image"])
	}

	for _, key := range []string{"scheduler", "enable_hr"} {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present in the NovelAI payload, want it dropped", key)
		}
	}
}

func TestImg2ImgForgeIgnoresNoise(t *testing.T) {
	forgeLog := &requestLog{}

	g := New(newForgeBackend(t, forgeLog), nil)

	req := Img2ImgRequest{
		Params:    Params{Model: "sd_xl_base_1.0.safetensors", Prompt: "a cat", Steps: 20, Width: 512, Height: 512, CFGScale: 7, Seed: -1},
		InitImage: []byte("\x89PNG\r\n\x1a\n"),
		Noise:     0.1,
	}

	if _, err := g.Img2Img(context.Background(), req); err != nil {
		t.Fatalf("Img2Img() error: %v", err)
	}

	_, bodies := forgeLog.snapshot()
	if body := decodeJSONBody(t, bodies[0]); body["noise"] != nil {
		t.Errorf("noise = %v, want it dropped", body["noise"])
	}
}

func TestImg2ImgForgeMapsStrength(t *testing.T) {
	forgeLog := &requestLog{}

	g := New(newForgeBackend(t, forgeLog), nil)

	req := Img2ImgRequest{
		Params:    Params{Model: "sd_xl_base_1.0.safetensors", Prompt: "a cat", Steps: 20, Width: 512, Height: 512, CFGScale: 7, Seed: -1},
		InitImage: []byte("\x89PNG\r\n\x1a\n"),
		Strength:  0.35,
	}

	if _, err := g.Img2Img(context.Background(), req); err != nil {
		t.Fatalf("Img2Img() error: %v", err)
	}

	_, bodies := forgeLog.snapshot()
	body := decodeJSONBody(t, bodies[0])

	if got := numberField(t, body, "denoising_strength"); got != 0.35 {
		t.Errorf("denoising_strength = %v, want 0.35", got)
	}
}
