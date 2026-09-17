package novelai

import "testing"

func TestIsModel(t *testing.T) {
	models := []string{
		"nai-diffusion-5-full",
		"nai-diffusion-4-full",
		"nai-diffusion-5-anime",
		"nai-diffusion-furry-3",
		"nai-diffusion-9-experimental",
		"nai-diffusion-4-full.safetensors",
		"  nai-diffusion-5-full  ",
	}

	for _, model := range models {
		if !IsModel(model) {
			t.Errorf("IsModel(%q) = false, want true", model)
		}
	}
}

func TestIsModelRejectsOtherModels(t *testing.T) {
	models := []string{
		"",
		"sd_xl_base_1.0.safetensors",
		"nai_diffusion_5_full",
		"model-nai-diffusion-5-full",
	}

	for _, model := range models {
		if IsModel(model) {
			t.Errorf("IsModel(%q) = true, want false", model)
		}
	}
}
