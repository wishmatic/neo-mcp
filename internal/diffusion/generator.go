package diffusion

import (
	"context"
	"errors"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/novelai"
)

const (
	DefaultSampler   = "DPM++ 2M"
	DefaultScheduler = "Automatic"

	ProviderForge   = "forge"
	ProviderNovelAI = "novelai"
)

var (
	ErrForgeNotConfigured = errors.New("the Forge backend is not configured")

	BgkillModels = forge.BgkillModels
)

type Client struct {
	forge   *forge.Client
	novelai *novelai.Client
}

func New(forge *forge.Client, novelai *novelai.Client) *Client {
	return &Client{forge: forge, novelai: novelai}
}

func ProviderOf(model string) string {
	if novelai.IsModel(model) {
		return ProviderNovelAI
	}

	return ProviderForge
}

func (g *Client) ForgeEnabled() bool {
	return g.forge != nil
}

func (g *Client) NovelAIEnabled() bool {
	return g.novelai != nil
}

func (g *Client) Txt2Img(ctx context.Context, req Txt2ImgRequest) ([][]byte, error) {
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

func (g *Client) Img2Img(ctx context.Context, req Img2ImgRequest) ([][]byte, error) {
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

func (g *Client) Bgkill(ctx context.Context, req BgkillRequest) ([]byte, error) {
	if g.forge == nil {
		return nil, ErrForgeNotConfigured
	}

	return g.forge.Bgkill(ctx, forgeBgkillRequest(req))
}

func noNovelAIClientError(model string) error {
	return fmt.Errorf("model %q requires the NovelAI backend, but NOVELAI_API_KEY is not set", model)
}
