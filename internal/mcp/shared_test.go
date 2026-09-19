package mcp

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
)

func schemas() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"txt2img": txt2imgSchema(imgfmt.Default),
		"img2img": img2imgSchema(imgfmt.Default),
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
		"model",
		"forge_preset",
		"vae_and_text_models",
		"prompt",
		"negative_prompt",
		"sampler_name",
		"scheduler",
		"steps",
		"width",
		"height",
		"cfg_scale",
		"seed",
		"enable_hr",
		"hr_scale",
		"hr_upscaler",
		"hr_second_pass_steps",
		"hr_cfg",
		"format",
		"nsfw",
		"return_as",
	}

	for name, s := range schemas() {
		for _, field := range shared {
			if s.Properties[field] == nil {
				t.Errorf("%s: missing shared field %q", name, field)
			}
		}
	}
}

func TestFormatFlagSchema(t *testing.T) {
	withFlag := schemas()
	withFlag["bgkill"] = bgkillSchema(imgfmt.Default)

	for tool, s := range withFlag {
		prop := s.Properties["format"]
		if prop == nil {
			t.Errorf("%s: format property is missing", tool)
			continue
		}

		if prop.Type != "string" {
			t.Errorf("%s: format type = %q, want string", tool, prop.Type)
		}

		names := imgfmt.Names()
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
	for _, name := range imgfmt.Names() {
		format, err := imgfmt.Parse(name)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", name, err)
		}

		want := `"` + name + `"`

		for tool, prop := range map[string]*jsonschema.Schema{
			"txt2img": txt2imgSchema(format).Properties["format"],
			"img2img": img2imgSchema(format).Properties["format"],
			"bgkill":  bgkillSchema(format).Properties["format"],
		} {
			if string(prop.Default) != want {
				t.Errorf("%s with default %s: format default = %s, want %s", tool, name, prop.Default, want)
			}
		}
	}
}

func TestReturnAsFlagSchema(t *testing.T) {
	withFlag := schemas()
	withFlag["bgkill"] = bgkillSchema(imgfmt.Default)

	want := []any{string(returnURL), string(returnImage)}

	for tool, s := range withFlag {
		prop := s.Properties["return_as"]
		if prop == nil {
			t.Errorf("%s: return_as property is missing", tool)
			continue
		}

		if prop.Type != "string" {
			t.Errorf("%s: return_as type = %q, want string", tool, prop.Type)
		}

		if !slices.Equal(prop.Enum, want) {
			t.Errorf("%s: return_as enum = %v, want %v", tool, prop.Enum, want)
		}

		if prop.Default != nil {
			t.Errorf("%s: return_as default = %s, want none", tool, prop.Default)
		}

		if !slices.Contains(s.Required, "return_as") {
			t.Errorf("%s: return_as is not required", tool)
		}
	}
}

func TestParseReturnMode(t *testing.T) {
	tests := map[string]returnMode{
		"":        returnURL,
		"url":     returnURL,
		"  URL  ": returnURL,
		"image":   returnImage,
		"IMAGE":   returnImage,
	}

	for value, want := range tests {
		got, err := parseReturnMode(value)
		if err != nil || got != want {
			t.Errorf("parseReturnMode(%q) = %q, %v, want %q", value, got, err, want)
		}
	}

	if _, err := parseReturnMode("inline"); err == nil {
		t.Error("parseReturnMode(inline) error = nil, want an error")
	}
}

func TestModelDescriptionRoutesNovelAI(t *testing.T) {
	for name, s := range schemas() {
		model := s.Properties["model"]

		if !strings.Contains(model.Description, "nai-diffusion-") {
			t.Errorf("%s: model description %q does not mention nai-diffusion-", name, model.Description)
		}

		if len(model.Enum) != 0 {
			t.Errorf("%s: model enum = %v, want none", name, model.Enum)
		}
	}
}

func TestSamplerDefaultsAreEmpty(t *testing.T) {
	for name, s := range schemas() {
		for _, field := range []string{"sampler_name", "scheduler"} {
			raw := s.Properties[field].Default

			if raw == nil || string(raw) != `""` {
				t.Errorf("%s: %s default = %s, want an empty string", name, field, raw)
			}
		}
	}
}

func TestNSFWFlagSchema(t *testing.T) {
	for name, s := range schemas() {
		prop := s.Properties["nsfw"]
		if prop == nil {
			t.Fatalf("%s: nsfw property is missing", name)
		}

		if prop.Type != "boolean" {
			t.Errorf("%s: nsfw type = %q, want boolean", name, prop.Type)
		}

		if string(prop.Default) != "false" {
			t.Errorf("%s: nsfw default = %s, want false", name, prop.Default)
		}

		if slices.Contains(s.Required, "nsfw") {
			t.Errorf("%s: nsfw must not be required", name)
		}
	}
}

func TestImg2ImgNoiseDefault(t *testing.T) {
	noise := img2imgSchema(imgfmt.Default).Properties["noise"]
	if noise == nil {
		t.Fatal("noise property is missing")
	}

	if string(noise.Default) != "0" {
		t.Errorf("noise default = %s, want 0", noise.Default)
	}

	if txt2imgSchema(imgfmt.Default).Properties["noise"] != nil {
		t.Error("txt2img has a noise property, want none")
	}
}
