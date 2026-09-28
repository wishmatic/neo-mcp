package mcp

import (
	"slices"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/diffusion"
	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"go.uber.org/zap"
)

func TestNewRegistersTools(t *testing.T) {
	srv, err := New(Deps{Log: zapNop()})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}
}

func TestToolRegistration(t *testing.T) {
	tests := []struct {
		name    string
		forge   *forge.Client
		novelai *novelai.Client
		want    []string
	}{
		{
			name:    "novelai only",
			novelai: novelai.New("http://example.com", "sk"),
			want:    []string{"txt2img", "img2img", "crop", "convert"},
		},
		{
			name:  "forge only",
			forge: forge.New("http://example.com", zapNop()),
			want:  []string{"txt2img", "img2img", "bgkill", "crop", "convert"},
		},
		{
			name: "none",
			want: []string{"crop", "convert"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := New(Deps{
				Log:       zapNop(),
				Generator: diffusion.New(tt.forge, tt.novelai),
			})
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			got := toolNames(t, srv)
			want := append([]string(nil), tt.want...)

			slices.Sort(got)
			slices.Sort(want)

			if !slices.Equal(got, want) {
				t.Errorf("tools = %v, want %v", got, want)
			}
		})
	}
}

func zapNop() *zap.Logger {
	return zap.NewNop()
}

func TestGenerationDescriptionsRequireDenoisingStrengthForUpscaling(t *testing.T) {
	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: diffusion.New(forge.New("http://example.com", zapNop()), nil),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	for _, name := range []string{"txt2img", "img2img"} {
		desc := toolByName(t, srv, name).Description

		if !strings.Contains(desc, "When HR upscaling is enabled, denoising_strength is required") {
			t.Errorf("%s description %q does not state that HR upscaling requires denoising_strength", name, desc)
		}
	}
}
