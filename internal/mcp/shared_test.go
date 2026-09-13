package mcp

import (
	"slices"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func schemas() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"txt2img": txt2imgSchema(),
		"img2img": img2imgSchema(),
	}
}

func TestModelIsRequired(t *testing.T) {
	for name, s := range schemas() {
		if !slices.Contains(s.Required, "model") {
			t.Errorf("%s: model is not required", name)
		}

		model := s.Properties["model"]
		if model == nil {
			t.Errorf("%s: model property is missing", name)
			continue
		}

		if model.Default != nil {
			t.Errorf("%s: model must not have a default", name)
		}
	}
}

func TestSchemasIncludeSharedFields(t *testing.T) {
	shared := []string{
		"model", "forge_preset", "vae_and_text_models", "prompt", "negative_prompt",
		"sampler_name", "scheduler", "steps", "width", "height", "cfg_scale", "seed",
		"enable_hr", "hr_scale", "hr_upscaler", "hr_second_pass_steps", "hr_cfg",
	}

	for name, s := range schemas() {
		for _, field := range shared {
			if s.Properties[field] == nil {
				t.Errorf("%s: missing shared field %q", name, field)
			}
		}
	}
}
