package imagegen

import (
	"context"
	"errors"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

const (
	DefaultSampler   = "DPM++ 2M"
	DefaultScheduler = "Automatic"
)

var ErrForgeNotConfigured = errors.New("the Forge backend is not configured")

type Generator struct {
	forge   *sdwebui.Client
	novelai *novelai.Client
}

func New(forge *sdwebui.Client, novelai *novelai.Client) *Generator {
	return &Generator{forge: forge, novelai: novelai}
}

func ProviderOf(model string) string {
	if novelai.IsModel(model) {
		return "novelai"
	}

	return "forge"
}

func (g *Generator) Txt2Img(ctx context.Context, req Txt2ImgRequest) ([][]byte, error) {
	if novelai.IsModel(req.Model) {
		if g.novelai == nil {
			return nil, noNovelAIClientError(req.Model)
		}

		return g.novelai.Txt2Img(ctx, novelaiTxt2ImgRequest(req))
	}

	if g.forge == nil {
		return nil, ErrForgeNotConfigured
	}

	return g.forge.Txt2Img(ctx, forgeTxt2ImgRequest(req))
}

func (g *Generator) Img2Img(ctx context.Context, req Img2ImgRequest) ([][]byte, error) {
	if novelai.IsModel(req.Model) {
		if g.novelai == nil {
			return nil, noNovelAIClientError(req.Model)
		}

		return g.novelai.Img2Img(ctx, novelaiImg2ImgRequest(req))
	}

	if g.forge == nil {
		return nil, ErrForgeNotConfigured
	}

	return g.forge.Img2Img(ctx, forgeImg2ImgRequest(req))
}

func noNovelAIClientError(model string) error {
	return fmt.Errorf("model %q requires the NovelAI backend, but NOVELAI_API_KEY is not set", model)
}
