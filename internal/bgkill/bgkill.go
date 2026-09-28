package bgkill

import (
	"context"

	"github.com/wishmatic/neo-mcp/internal/forge"
)

var Models = []string{
	"General",
	"General-HR",
	"General-Lite",
	"General-Lite-2K",
	"Portrait",
	"Matting",
	"Matting-HR",
	"Matting-Lite",
	"Anime-Lite",
	"Dynamic",
	"DIS",
	"HRSOD",
	"COD",
	"DIS-TR_TEs",
}

type Request struct {
	ModelName  string
	ImageData  []byte
	IsFullMode bool
}

type Service struct {
	forge *forge.Client
}

func New(forge *forge.Client) *Service {
	return &Service{forge: forge}
}

func (s *Service) Enabled() bool {
	return s.forge != nil
}

func (s *Service) Remove(ctx context.Context, req Request) ([]byte, error) {
	return s.forge.Bgkill(ctx, forge.BgkillRequest{
		ModelName:  req.ModelName,
		ImageData:  req.ImageData,
		IsFullMode: req.IsFullMode,
	})
}
