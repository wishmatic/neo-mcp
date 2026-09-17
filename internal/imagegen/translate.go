package imagegen

import (
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

func forgeTxt2ImgRequest(req Txt2ImgRequest) sdwebui.Txt2ImgRequest {
	return sdwebui.Txt2ImgRequest{
		Checkpoint:             req.Model,
		ForgePreset:            req.ForgePreset,
		ForgeAdditionalModules: req.VAEAndTextModels,

		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Steps:          req.Steps,
		Width:          req.Width,
		Height:         req.Height,
		Seed:           req.Seed,
		CFGScale:       req.CFGScale,
		SamplerName:    orDefault(req.Sampler, DefaultSampler),
		Scheduler:      orDefault(req.Scheduler, DefaultScheduler),

		EnableHR:          req.EnableHR,
		HRScale:           req.HRScale,
		HRUpscaler:        req.HRUpscaler,
		HRSecondPassSteps: req.HRSecondPassSteps,
		DenoisingStrength: req.HRDenoisingStrength,
		HRCFGScale:        req.HRCFGScale,
	}
}

func forgeImg2ImgRequest(req Img2ImgRequest) sdwebui.Img2ImgRequest {
	return sdwebui.Img2ImgRequest{
		Checkpoint:             req.Model,
		ForgePreset:            req.ForgePreset,
		ForgeAdditionalModules: req.VAEAndTextModels,

		InitImageData: req.InitImage,

		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Steps:          req.Steps,
		Width:          req.Width,
		Height:         req.Height,
		Seed:           req.Seed,
		CFGScale:       req.CFGScale,
		SamplerName:    orDefault(req.Sampler, DefaultSampler),
		Scheduler:      orDefault(req.Scheduler, DefaultScheduler),

		DenoisingStrength: req.Strength,

		EnableHR:          req.EnableHR,
		HRScale:           req.HRScale,
		HRUpscaler:        req.HRUpscaler,
		HRSecondPassSteps: req.HRSecondPassSteps,
		HRCFGScale:        req.HRCFGScale,
	}
}

func novelaiTxt2ImgRequest(req Txt2ImgRequest) novelai.Txt2ImgRequest {
	return novelai.Txt2ImgRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Sampler:        req.Sampler,
		Steps:          req.Steps,
		Width:          req.Width,
		Height:         req.Height,
		Scale:          req.CFGScale,
		Seed:           req.Seed,
	}
}

func novelaiImg2ImgRequest(req Img2ImgRequest) novelai.Img2ImgRequest {
	return novelai.Img2ImgRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Sampler:        req.Sampler,
		Steps:          req.Steps,
		Width:          req.Width,
		Height:         req.Height,
		Scale:          req.CFGScale,
		Seed:           req.Seed,
		InitImage:      req.InitImage,
		Strength:       req.Strength,
		Noise:          req.Noise,
	}
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
