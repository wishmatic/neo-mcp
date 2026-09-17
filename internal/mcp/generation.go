package mcp

import "github.com/wishmatic/neo-mcp/internal/imagegen"

func imagegenTxt2ImgRequest(in txt2imgInput) imagegen.Txt2ImgRequest {
	return imagegen.Txt2ImgRequest{
		Params:              imagegenParams(in.generationInput),
		HRDenoisingStrength: in.DenoisingStrength,
	}
}

func imagegenImg2ImgRequest(in img2imgInput, initImage []byte) imagegen.Img2ImgRequest {
	return imagegen.Img2ImgRequest{
		Params:    imagegenParams(in.generationInput),
		InitImage: initImage,
		Strength:  in.DenoisingStrength,
		Noise:     in.Noise,
	}
}

func imagegenParams(in generationInput) imagegen.Params {
	return imagegen.Params{
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
