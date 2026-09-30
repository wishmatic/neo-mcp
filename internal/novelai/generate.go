package novelai

import (
	"context"
	"fmt"
	"math"

	"github.com/wishmatic/neo-mcp/internal/format"
)

const (
	defaultDimension = 512
	defaultStrength  = 0.75
	gridStep         = 8
)

// GenerateRequest is one generation to run. An unset InitImage generates from the prompt alone; a set one transforms
// that image instead, which is also the only case that reads Strength and Noise. Width and Height left at 0 are filled
// in from the init image, or with 512 pixels each without one.
type GenerateRequest struct {
	InitImage []byte

	Model          string
	Prompt         string
	NegativePrompt string
	Sampler        string
	Steps          int

	Width  int
	Height int
	Scale  float64

	Strength float64
	Noise    float64

	Seed int
}

func (c *Client) Generate(ctx context.Context, req GenerateRequest) ([][]byte, error) {
	width, height, err := outputSize(req)
	if err != nil {
		return nil, err
	}

	if len(req.InitImage) == 0 {
		return c.txt2img(ctx, txt2imgRequest{
			Model:          req.Model,
			Prompt:         req.Prompt,
			NegativePrompt: req.NegativePrompt,
			Sampler:        req.Sampler,
			Steps:          req.Steps,
			Width:          width,
			Height:         height,
			Scale:          req.Scale,
			Seed:           req.Seed,
		})
	}

	strength := req.Strength
	if strength == 0 {
		strength = defaultStrength
	}

	return c.img2img(ctx, img2imgRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Sampler:        req.Sampler,
		Steps:          req.Steps,
		Width:          width,
		Height:         height,
		Scale:          req.Scale,
		Seed:           req.Seed,
		InitImage:      req.InitImage,
		Strength:       strength,
		Noise:          req.Noise,
	})
}

// outputSize fills in whichever of width and height the caller left out: 512 each without an init image, and otherwise
// the init image's size, both of them when neither is set and the missing one from the init image's aspect ratio when
// only one is set. Derived sizes land on the eight pixel grid first, so NovelAI's round up to a multiple of 64 stays
// as close to what the caller was told as its own grid allows.
func outputSize(req GenerateRequest) (int, int, error) {
	if len(req.InitImage) == 0 {
		return dimensionOrDefault(req.Width), dimensionOrDefault(req.Height), nil
	}

	if req.Width != 0 && req.Height != 0 {
		return req.Width, req.Height, nil
	}

	width, height, err := format.Dimensions(req.InitImage)
	if err != nil {
		return 0, 0, fmt.Errorf("read init image size: %w (pass width and height to override)", err)
	}

	switch {
	case req.Width == 0 && req.Height == 0:
		return gridSize(width), gridSize(height), nil
	case req.Width == 0:
		return gridSize(aspectDimension(req.Height, width, height)), req.Height, nil
	default:
		return req.Width, gridSize(aspectDimension(req.Width, height, width)), nil
	}
}

func dimensionOrDefault(value int) int {
	if value == 0 {
		return defaultDimension
	}

	return value
}

func aspectDimension(known, otherSource, knownSource int) int {
	return int(math.Round(float64(known) * float64(otherSource) / float64(knownSource)))
}

func gridSize(value int) int {
	return max(gridStep, value/gridStep*gridStep)
}
