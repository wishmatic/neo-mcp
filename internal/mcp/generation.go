package mcp

import "github.com/wishmatic/neo-mcp/internal/generation"

func generationTxt2ImgRequest(in txt2imgInput) generation.Txt2ImgRequest {
	return generation.Txt2ImgRequest{
		Params:              generationParams(in.generationInput),
		HRDenoisingStrength: in.DenoisingStrength,
	}
}

func generationImg2ImgRequest(in img2imgInput, initImage []byte) generation.Img2ImgRequest {
	return generation.Img2ImgRequest{
		Params:    generationParams(in.generationInput),
		InitImage: initImage,
		Strength:  in.DenoisingStrength,
		Noise:     in.Noise,
	}
}

func generationParams(in generationInput) generation.Params {
	return generation.Params{
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
