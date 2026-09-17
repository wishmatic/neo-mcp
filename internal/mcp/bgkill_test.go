package mcp

import (
	"slices"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/crop"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

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

func TestBgkillCropOptions(t *testing.T) {
	var (
		five = 5
		zero = 0
	)

	tests := []struct {
		name string
		in   bgkillInput
		want crop.Options
		ok   bool
	}{
		{
			name: "crop disabled",
			in:   bgkillInput{},
		},
		{
			name: "crop only",
			in:   bgkillInput{IsCrop: true},
			want: crop.Options{},
			ok:   true,
		},
		{
			name: "square implies crop and defaults the padding",
			in:   bgkillInput{IsSquare: true},
			want: crop.Options{IsSquare: true, Padding: defaultSquarePadding},
			ok:   true,
		},
		{
			name: "square with explicit padding",
			in:   bgkillInput{IsCrop: true, IsSquare: true, Padding: &five},
			want: crop.Options{IsSquare: true, Padding: 5},
			ok:   true,
		},
		{
			name: "crop with explicit padding",
			in:   bgkillInput{IsCrop: true, Padding: &five},
			want: crop.Options{Padding: 5},
			ok:   true,
		},
		{
			name: "square with zero padding",
			in:   bgkillInput{IsCrop: true, IsSquare: true, Padding: &zero},
			want: crop.Options{IsSquare: true, Padding: 0},
			ok:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bgkillCropOptions(tt.in)
			if ok != tt.ok {
				t.Fatalf("bgkillCropOptions() ok = %v, want %v", ok, tt.ok)
			}

			if got != tt.want {
				t.Errorf("bgkillCropOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
