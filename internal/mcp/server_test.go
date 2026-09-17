package mcp

import (
	"slices"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"go.uber.org/zap"
)

func TestNewRegistersTools(t *testing.T) {
	srv, err := New(zapNop(), nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}
}

func TestToolRegistration(t *testing.T) {
	openaiClient, err := openai.New("http://example.com", "", "m", "")
	if err != nil {
		t.Fatalf("openai.New() error: %v", err)
	}

	tests := []struct {
		name     string
		forge    *sdwebui.Client
		novelai  *novelai.Client
		openai   *openai.Client
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
			name:   "openai only",
			openai: openaiClient,
			want:   []string{"img2txt"},
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
			srv, err := New(zapNop(), tt.forge, tt.novelai, tt.uploader, nil, nil, tt.openai)
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
