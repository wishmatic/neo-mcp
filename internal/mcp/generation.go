package mcp

import "github.com/wishmatic/neo-mcp/internal/diffusion"

func generationTxt2ImgRequest(in txt2imgInput) diffusion.Txt2ImgRequest {
	return diffusion.Txt2ImgRequest{
		Params:              generationParams(in.generationInput),
		HRDenoisingStrength: in.DenoisingStrength,
	}
}

func generationImg2ImgRequest(in img2imgInput, initImage []byte) diffusion.Img2ImgRequest {
	return diffusion.Img2ImgRequest{
		Params:    generationParams(in.generationInput),
		InitImage: initImage,
		Strength:  in.DenoisingStrength,
		Noise:     in.Noise,
	}
}

func generationParams(in generationInput) diffusion.Params {
	return diffusion.Params{
		Model:             in.Model,
		ForgePreset:       in.ForgePreset,
		VAEAndTextModels:  in.VAEAndTextModels,
		Prompt:            in.Prompt,
		NegativePrompt:    in.NegativePrompt,
		Sampler:           in.SamplingMethod,
		Scheduler:         in.ScheduleType,
		Steps:             in.SamplingSteps,
		Width:             in.Width,
		Height:            in.Height,
		CFGScale:          in.CFGScale,
		Seed:              in.Seed,
		EnableHR:          in.EnableHR,
		HRScale:           in.HRScale,
		HRUpscaler:        in.HRUpscaler,
		HRSecondPassSteps: in.HRSecondPassSteps,
		HRCFGScale:        in.HRCFGScale,
	}
}
