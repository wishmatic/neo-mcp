package mcp

import (
	"slices"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
)

func TestBgkillRequestMapsInput(t *testing.T) {
	in := bgkillInput{
		ModelName:  "Portrait",
		ImageURL:   "https://example.com/a.png",
		IsFullMode: true,
	}

	got := bgkillRequest(in, []byte("image"))

	if got.ModelName != "Portrait" || got.IsFullMode != true {
		t.Errorf("request = %+v, want the input fields copied", got)
	}

	if string(got.ImageData) != "image" {
		t.Errorf("image = %q, want it copied", got.ImageData)
	}
}

func TestBgkillSchema(t *testing.T) {
	s := bgkillSchema(imgfmt.Default)

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

	if len(model.Enum) != len(bgkill.Models) {
		t.Errorf("bgkill: model_name has %d enum values, want %d", len(model.Enum), len(bgkill.Models))
	}
}

func TestBgkillSchemaDefaults(t *testing.T) {
	s := bgkillSchema(imgfmt.Default)

	fullMode := s.Properties["full_mode"]
	if fullMode == nil {
		t.Fatal("bgkill: full_mode property is missing")
	}

	if string(fullMode.Default) != "false" {
		t.Errorf("bgkill: full_mode default = %s, want false", fullMode.Default)
	}

	for _, field := range []string{"crop", "square", "circle", "padding"} {
		if s.Properties[field] != nil {
			t.Errorf("bgkill: %q should no longer be an input", field)
		}
	}
}
