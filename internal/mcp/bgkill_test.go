package mcp

import (
	"slices"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/format"
)

func TestBgkillSchema(t *testing.T) {
	s := bgkillSchema(format.Default)

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

	if len(model.Enum) != len(forge.BgkillModels) {
		t.Errorf("bgkill: model_name has %d enum values, want %d", len(model.Enum), len(forge.BgkillModels))
	}
}

func TestBgkillSchemaDefaults(t *testing.T) {
	s := bgkillSchema(format.Default)

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
