package mcp

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/present"
)

func schemas() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"forge":   forgeSchema(format.Default),
		"novelai": novelaiSchema(format.Default),
	}
}

func allImageSchemas() map[string]*jsonschema.Schema {
	all := schemas()
	all["bgkill"] = bgkillSchema(format.Default)
	all["edit"] = editSchema(format.Default)
	all["convert"] = convertSchema()

	return all
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

func TestPromptIsRequired(t *testing.T) {
	for name, s := range schemas() {
		if !slices.Contains(s.Required, "prompt") {
			t.Errorf("%s: prompt is not required", name)
		}
	}
}

func TestInitImageIsOptional(t *testing.T) {
	for name, s := range schemas() {
		if slices.Contains(s.Required, "init_image_url") {
			t.Errorf("%s: init_image_url must not be required, as it is what selects img2img", name)
		}
	}
}

func TestSchemasIncludeSharedGenerationFields(t *testing.T) {
	shared := []string{
		"prompt",
		"negative_prompt",
		"sampler_name",
		"steps",
		"width",
		"height",
		"cfg_scale",
		"seed",
		"init_image_url",
		"denoising_strength",
		"format",
		"inline_max_edge",
		"inline_max_bytes",
	}

	for name, s := range schemas() {
		for _, field := range shared {
			if s.Properties[field] == nil {
				t.Errorf("%s: missing shared field %q", name, field)
			}
		}
	}
}

func TestForgeOnlyFieldsAreForgeOnly(t *testing.T) {
	fields := []string{
		"forge_preset",
		"vae_and_text_models",
		"scheduler",
		"enable_hr",
		"hr_scale",
		"hr_upscaler",
		"hr_second_pass_steps",
		"hr_cfg",
	}

	for _, field := range fields {
		if forgeSchema(format.Default).Properties[field] == nil {
			t.Errorf("forge: missing %q", field)
		}

		if novelaiSchema(format.Default).Properties[field] != nil {
			t.Errorf("novelai: %q is present, want it gone with the provider routing", field)
		}
	}
}

func TestForgePresetIsRequired(t *testing.T) {
	if !slices.Contains(forgeSchema(format.Default).Required, "forge_preset") {
		t.Error("forge: forge_preset is not required")
	}
}

func TestNoiseIsNovelAIOnly(t *testing.T) {
	noise := novelaiSchema(format.Default).Properties["noise"]
	if noise == nil {
		t.Fatal("novelai: noise property is missing")
	}

	if string(noise.Default) != "0" {
		t.Errorf("novelai: noise default = %s, want 0", noise.Default)
	}

	if forgeSchema(format.Default).Properties["noise"] != nil {
		t.Error("forge: noise is present, want none")
	}
}

func TestDimensionsHaveNoDefault(t *testing.T) {
	for name, s := range schemas() {
		for _, field := range []string{"width", "height"} {
			if s.Properties[field].Default != nil {
				t.Errorf("%s: %s default = %s, want none so the init image can size it",
					name, field, s.Properties[field].Default)
			}
		}
	}
}

func TestDenoisingStrengthHasNoDefault(t *testing.T) {
	for name, s := range schemas() {
		prop := s.Properties["denoising_strength"]

		if prop.Type != "number" {
			t.Errorf("%s: denoising_strength type = %q, want number", name, prop.Type)
		}

		if prop.Default != nil {
			t.Errorf("%s: denoising_strength default = %s, want none: 0 is what an omitted value looks like, and it "+
				"means different things on the txt2img and img2img paths", name, prop.Default)
		}
	}
}

func TestForgeDenoisingStrengthDescribesBothMeanings(t *testing.T) {
	desc := forgeSchema(format.Default).Properties["denoising_strength"].Description

	for _, want := range []string{"init image", "0.75", "hi-res second pass"} {
		if !strings.Contains(desc, want) {
			t.Errorf("forge: denoising_strength description %q does not mention %q", desc, want)
		}
	}
}

func TestInlineBudgetSchema(t *testing.T) {
	tests := map[string]int{
		"inline_max_edge":  present.DefaultInlineMaxEdge,
		"inline_max_bytes": present.DefaultInlineMaxBytes,
	}

	for tool, s := range allImageSchemas() {
		for field, def := range tests {
			prop := s.Properties[field]
			if prop == nil {
				t.Errorf("%s: %s is missing", tool, field)
				continue
			}

			if got := string(prop.Default); got != strconv.Itoa(def) {
				t.Errorf("%s: %s default = %s, want %d", tool, field, got, def)
			}

			if slices.Contains(s.Required, field) {
				t.Errorf("%s: %s must be optional, as naming no budget takes the default", tool, field)
			}
		}
	}
}

func TestFormatFlagSchema(t *testing.T) {
	withFlag := schemas()
	withFlag["bgkill"] = bgkillSchema(format.Default)

	for tool, s := range withFlag {
		prop := s.Properties["format"]
		if prop == nil {
			t.Errorf("%s: format property is missing", tool)
			continue
		}

		if prop.Type != "string" {
			t.Errorf("%s: format type = %q, want string", tool, prop.Type)
		}

		names := format.Names()
		if len(prop.Enum) != len(names) {
			t.Errorf("%s: format enum = %v, want %v", tool, prop.Enum, names)
		}

		for i, name := range names {
			if i < len(prop.Enum) && prop.Enum[i] != name {
				t.Errorf("%s: format enum[%d] = %v, want %q", tool, i, prop.Enum[i], name)
			}
		}

		if string(prop.Default) != `"webp"` {
			t.Errorf("%s: format default = %s, want webp", tool, prop.Default)
		}

		if slices.Contains(s.Required, "format") {
			t.Errorf("%s: format must not be required", tool)
		}
	}
}

func TestFormatFlagSchemaFollowsConfiguredDefault(t *testing.T) {
	for _, name := range format.Names() {
		format, err := format.Parse(name)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", name, err)
		}

		want := `"` + name + `"`

		for tool, prop := range map[string]*jsonschema.Schema{
			"forge":   forgeSchema(format).Properties["format"],
			"novelai": novelaiSchema(format).Properties["format"],
			"bgkill":  bgkillSchema(format).Properties["format"],
		} {
			if string(prop.Default) != want {
				t.Errorf("%s with default %s: format default = %s, want %s", tool, name, prop.Default, want)
			}
		}
	}
}

func TestSamplerDefaultsAreTheToolDefaults(t *testing.T) {
	defaults := map[string]string{
		"forge":   forgeDefaultSampler,
		"novelai": novelai.DefaultSampler,
	}

	for name, s := range schemas() {
		want := `"` + defaults[name] + `"`

		if string(s.Properties["sampler_name"].Default) != want {
			t.Errorf("%s: sampler_name default = %s, want %s", name, s.Properties["sampler_name"].Default, want)
		}
	}
}
