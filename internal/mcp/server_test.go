package mcp

import (
	"slices"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
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
		name     string
		forge    *sdwebui.Client
		novelai  *novelai.Client
		uploader *s3upload.Client
		want     []string
	}{
		{
			name:    "novelai only",
			novelai: novelai.New("http://example.com", "sk", false),
			want:    []string{"txt2img", "img2img", "anlas"},
		},
		{
			name:  "forge only",
			forge: sdwebui.New("http://example.com", false),
			want:  []string{"txt2img", "img2img", "bgkill"},
		},
		{
			name:     "uploader only",
			uploader: newPublicizeUploader(t, &[]uploadCapture{}),
			want:     []string{"publicize"},
		},
		{
			name: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := New(Deps{
				Log:       zapNop(),
				Generator: imagegen.New(tt.forge, tt.novelai),
				Bgkill:    bgkill.New(tt.forge),
				Publisher: publish.New(tt.uploader, nil, zapNop()),
				NovelAI:   tt.novelai,
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
