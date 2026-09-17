package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

var errNoForgeClient = errors.New("the Forge backend is not configured")

func providerOf(model string) string {
	if novelai.IsModel(model) {
		return "novelai"
	}

	return "forge"
}

func (h *handlers) generateTxt2Img(ctx context.Context, in txt2imgInput) ([][]byte, error) {
	if !novelai.IsModel(in.Model) {
		if h.forge == nil {
			return nil, errNoForgeClient
		}

		return h.forge.Txt2Img(ctx, forgeTxt2ImgRequest(in))
	}

	if h.novelai == nil {
		return nil, noNovelAIClientError(in.Model)
	}

	return h.novelai.Txt2Img(ctx, novelaiTxt2ImgRequest(in))
}

func (h *handlers) generateImg2Img(ctx context.Context, in img2imgInput, initImage []byte) ([][]byte, error) {
	if !novelai.IsModel(in.Model) {
		if h.forge == nil {
			return nil, errNoForgeClient
		}

		return h.forge.Img2Img(ctx, forgeImg2ImgRequest(in, initImage))
	}

	if h.novelai == nil {
		return nil, noNovelAIClientError(in.Model)
	}

	return h.novelai.Img2Img(ctx, novelaiImg2ImgRequest(in, initImage))
}

func noNovelAIClientError(model string) error {
	return fmt.Errorf("model %q requires the NovelAI backend, but NOVELAI_API_KEY is not set", model)
}

func forgeTxt2ImgRequest(in txt2imgInput) sdwebui.Txt2ImgRequest {
	return sdwebui.Txt2ImgRequest{
		Checkpoint:             in.Model,
		ForgePreset:            in.ForgePreset,
		ForgeAdditionalModules: in.VAEAndTextModels,

		Prompt:         in.Prompt,
		NegativePrompt: in.NegativePrompt,
		Steps:          in.SamplingSteps,
		Width:          in.Width,
		Height:         in.Height,
		Seed:           in.Seed,
		CFGScale:       in.CFGScale,
		SamplerName:    orDefault(in.SamplingMethod, defaultSampler),
		Scheduler:      orDefault(in.ScheduleType, defaultScheduler),

		EnableHR:          in.EnableHR,
		HRScale:           in.HRScale,
		HRUpscaler:        in.HRUpscaler,
		HRSecondPassSteps: in.HRSecondPassSteps,
		DenoisingStrength: in.DenoisingStrength,
		HRCFGScale:        in.HRCFGScale,
	}
}

func forgeImg2ImgRequest(in img2imgInput, initImage []byte) sdwebui.Img2ImgRequest {
	return sdwebui.Img2ImgRequest{
		Checkpoint:             in.Model,
		ForgePreset:            in.ForgePreset,
		ForgeAdditionalModules: in.VAEAndTextModels,

		InitImageData: initImage,

		Prompt:         in.Prompt,
		NegativePrompt: in.NegativePrompt,
		Steps:          in.SamplingSteps,
		Width:          in.Width,
		Height:         in.Height,
		Seed:           in.Seed,
		CFGScale:       in.CFGScale,
		SamplerName:    orDefault(in.SamplingMethod, defaultSampler),
		Scheduler:      orDefault(in.ScheduleType, defaultScheduler),

		DenoisingStrength: in.DenoisingStrength,

		EnableHR:          in.EnableHR,
		HRScale:           in.HRScale,
		HRUpscaler:        in.HRUpscaler,
		HRSecondPassSteps: in.HRSecondPassSteps,
		HRCFGScale:        in.HRCFGScale,
	}
}

func novelaiTxt2ImgRequest(in txt2imgInput) novelai.Txt2ImgRequest {
	return novelai.Txt2ImgRequest{
		Model:          in.Model,
		Prompt:         in.Prompt,
		NegativePrompt: in.NegativePrompt,
		Sampler:        in.SamplingMethod,
		Steps:          in.SamplingSteps,
		Width:          in.Width,
		Height:         in.Height,
		Scale:          in.CFGScale,
		Seed:           in.Seed,
	}
}

func novelaiImg2ImgRequest(in img2imgInput, initImage []byte) novelai.Img2ImgRequest {
	return novelai.Img2ImgRequest{
		Model:          in.Model,
		Prompt:         in.Prompt,
		NegativePrompt: in.NegativePrompt,
		Sampler:        in.SamplingMethod,
		Steps:          in.SamplingSteps,
		Width:          in.Width,
		Height:         in.Height,
		Scale:          in.CFGScale,
		Seed:           in.Seed,
		InitImage:      initImage,
		Strength:       in.DenoisingStrength,
		Noise:          in.Noise,
	}
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
