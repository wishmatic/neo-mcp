package mcp

import (
	"slices"
	"testing"
)

func TestImagegenTxt2ImgRequestMapsInput(t *testing.T) {
	in := txt2imgInput{
		generationInput: generationInput{
			Model:             "m.safetensors",
			ForgePreset:       "krea",
			VAEAndTextModels:  []string{"vae.safetensors"},
			Prompt:            "p",
			NegativePrompt:    "n",
			SamplingMethod:    "Euler",
			ScheduleType:      "Karras",
			SamplingSteps:     30,
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
		DenoisingStrength: 0.4,
	}

	got := imagegenTxt2ImgRequest(in)

	if got.Model != "m.safetensors" || got.ForgePreset != "krea" ||
		!slices.Equal(got.VAEAndTextModels, []string{"vae.safetensors"}) {
		t.Errorf("model fields = %+v, want them copied", got.Params)
	}

	if got.Prompt != "p" || got.NegativePrompt != "n" {
		t.Errorf("prompt fields = %+v, want them copied", got.Params)
	}

	if got.Sampler != "Euler" || got.Scheduler != "Karras" || got.Steps != 30 {
		t.Errorf("sampling fields = %+v, want them copied", got.Params)
	}

	if got.Width != 768 || got.Height != 512 || got.CFGScale != 6.5 || got.Seed != 42 {
		t.Errorf("dimensions and seed = %+v, want them copied", got.Params)
	}

	if !got.EnableHR || got.HRScale != 2 || got.HRUpscaler != "4x" || got.HRSecondPassSteps != 12 ||
		got.HRCFGScale != 5 {
		t.Errorf("HR fields = %+v, want them copied", got.Params)
	}

	if got.HRDenoisingStrength != 0.4 {
		t.Errorf("HR denoising strength = %v, want 0.4", got.HRDenoisingStrength)
	}
}

func TestImagegenImg2ImgRequestMapsInput(t *testing.T) {
	initImage := []byte("\x89PNG\r\n\x1a\n")

	in := img2imgInput{
		generationInput:   generationInputFor("sd_xl_base_1.0.safetensors"),
		InitImageURL:      "https://example.com/a.png",
		DenoisingStrength: 0.6,
		Noise:             0.1,
	}

	got := imagegenImg2ImgRequest(in, initImage)

	if got.Model != "sd_xl_base_1.0.safetensors" || got.Prompt != "a cat" {
		t.Errorf("params = %+v, want them copied", got.Params)
	}

	if string(got.InitImage) != string(initImage) {
		t.Errorf("init image = %v, want it copied", got.InitImage)
	}

	if got.Strength != 0.6 || got.Noise != 0.1 {
		t.Errorf("strength = %v, noise = %v, want 0.6 and 0.1", got.Strength, got.Noise)
	}
}
