package novelai

import "context"

type Txt2ImgRequest struct {
	Model          string
	Prompt         string
	NegativePrompt string
	Sampler        string
	Steps          int
	Width          int
	Height         int
	Scale          float64
	Seed           int
}

func (c *Client) Txt2Img(ctx context.Context, req Txt2ImgRequest) ([][]byte, error) {
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
	params["stream"] = "msgpack"

	return c.generate(ctx, "txt2img", map[string]any{
		"input":      req.Prompt,
		"model":      req.Model,
		"action":     "generate",
		"parameters": params,
	})
}
