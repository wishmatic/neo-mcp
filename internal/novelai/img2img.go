package novelai

import (
	"context"
	"encoding/base64"
)

type Img2ImgRequest struct {
	Model          string
	Prompt         string
	NegativePrompt string
	Sampler        string
	Steps          int
	Width          int
	Height         int
	Scale          float64
	Seed           int
	InitImage      []byte
	Strength       float64
	Noise          float64
}

func (c *Client) Img2Img(ctx context.Context, req Img2ImgRequest) ([][]byte, error) {
	width, height, err := normalizeDimensions(req.Width, req.Height)
	if err != nil {
		return nil, err
	}

	params := baseParams(paramsInput{
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Sampler:        req.Sampler,
		Steps:          req.Steps,
		Width:          width,
		Height:         height,
		Scale:          req.Scale,
		Seed:           c.resolveSeed(req.Seed),
	})
	params["stream"] = streamMsgpack
	params["image"] = base64.StdEncoding.EncodeToString(req.InitImage)
	params["strength"] = req.Strength
	params["noise"] = req.Noise
	params["extra_noise_seed"] = c.randomSeed()

	return c.generate(ctx, "img2img", map[string]any{
		"input":      req.Prompt,
		"model":      req.Model,
		"action":     "img2img",
		"parameters": params,
	})
}
