package mcp

import (
	"slices"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/novelai"
)

func TestNewRegistersTools(t *testing.T) {
	srv, err := New(Clients{Log: zapNop()})
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
			want:    []string{"novelai", "edit", "convert"},
		},
		{
			name:  "forge only",
			forge: forge.New("http://example.com", zapNop()),
			want:  []string{"forge", "bgkill", "edit", "convert"},
		},
		{
			name:    "both",
			forge:   forge.New("http://example.com", zapNop()),
			novelai: novelai.New("http://example.com", "sk"),
			want:    []string{"forge", "novelai", "bgkill", "edit", "convert"},
		},
		{
			name: "none",
			want: []string{"edit", "convert"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := New(Clients{
				Log:     zapNop(),
				Forge:   tt.forge,
				NovelAI: tt.novelai,
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

func TestForgeDescriptionRequiresDenoisingStrengthForUpscaling(t *testing.T) {
	srv, err := New(Clients{
		Log:   zapNop(),
		Forge: forge.New("http://example.com", zapNop()),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	desc := toolByName(t, srv, "forge").Description

	if !strings.Contains(desc, "When HR upscaling is enabled, denoising_strength is required") {
		t.Errorf("forge description %q does not state that HR upscaling requires denoising_strength", desc)
	}
}

func TestGenerationDescriptionsDocumentTheInitImage(t *testing.T) {
	srv, err := New(Clients{
		Log:     zapNop(),
		Forge:   forge.New("http://example.com", zapNop()),
		NovelAI: novelai.New("http://example.com", "sk"),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	for _, name := range []string{"forge", "novelai"} {
		desc := toolByName(t, srv, name).Description

		for _, want := range []string{"init_image_url", "default to 512"} {
			if !strings.Contains(desc, want) {
				t.Errorf("%s description %q does not mention %q", name, desc, want)
			}
		}
	}
}
