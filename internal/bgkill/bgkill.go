package bgkill

import (
	"context"

	"github.com/wishmatic/neo-mcp/internal/crop"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
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
	IsCrop     bool
	IsSquare   bool
	Padding    *int
}

type Service struct {
	forge *sdwebui.Client
}

func New(forge *sdwebui.Client) *Service {
	return &Service{forge: forge}
}

func (s *Service) Enabled() bool {
	return s.forge != nil
}

func (s *Service) Remove(ctx context.Context, req Request) ([]byte, error) {
	foreground, err := s.forge.Bgkill(ctx, sdwebui.BgkillRequest{
		ModelName:  req.ModelName,
		ImageData:  req.ImageData,
		IsFullMode: req.IsFullMode,
	})
	if err != nil {
		return nil, err
	}

	opts, ok := crop.OptionsFor(req.IsCrop, req.IsSquare, req.Padding)
	if !ok {
		return foreground, nil
	}

	return crop.ToContent(foreground, opts)
}
