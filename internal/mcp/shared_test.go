package mcp

import (
	"slices"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
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

func TestBgkillSchema(t *testing.T) {
	s := bgkillSchema()

	for _, field := range []string{"model_name", "image_url"} {
		if !slices.Contains(s.Required, field) {
			t.Errorf("bgkill: %q is not required", field)
		}
	}

	model := s.Properties["model_name"]
	if model == nil {
		t.Fatal("bgkill: model_name property is missing")
	}

	if model.Default != nil {
		t.Error("bgkill: model_name must not have a default")
	}

	if len(model.Enum) != len(sdwebui.BgkillModels) {
		t.Errorf("bgkill: model_name has %d enum values, want %d", len(model.Enum), len(sdwebui.BgkillModels))
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
