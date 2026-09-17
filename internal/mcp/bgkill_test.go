package mcp

import (
	"slices"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/bgkill"
)

func TestBgkillRequestMapsInput(t *testing.T) {
	five := 5

	in := bgkillInput{
		ModelName:  "Portrait",
		ImageURL:   "https://example.com/a.png",
		IsFullMode: true,
		IsCrop:     true,
		IsSquare:   true,
		Padding:    &five,
	}

	got := bgkillRequest(in, []byte("image"))

	if got.ModelName != "Portrait" || got.IsFullMode != true || got.IsCrop != true || got.IsSquare != true {
		t.Errorf("request = %+v, want the input flags copied", got)
	}

	if string(got.ImageData) != "image" {
		t.Errorf("image = %q, want it copied", got.ImageData)
	}

	if got.Padding == nil || *got.Padding != 5 {
		t.Errorf("padding = %v, want 5", got.Padding)
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

	if len(model.Enum) != len(bgkill.Models) {
		t.Errorf("bgkill: model_name has %d enum values, want %d", len(model.Enum), len(bgkill.Models))
	}
}

func TestBgkillCropSchema(t *testing.T) {
	s := bgkillSchema()

	for _, field := range []string{"full_mode", "crop", "square"} {
		prop := s.Properties[field]
		if prop == nil {
			t.Fatalf("bgkill: %q property is missing", field)
		}

		if string(prop.Default) != "false" {
			t.Errorf("bgkill: %q default = %s, want false", field, prop.Default)
		}
	}

	padding := s.Properties["padding"]
	if padding == nil {
		t.Fatal("bgkill: padding property is missing")
	}

	if padding.Default != nil {
		t.Errorf("bgkill: padding default = %s, want none", padding.Default)
	}
}
