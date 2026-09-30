package forge

import (
	"context"
	"fmt"
	"math"

	"github.com/wishmatic/neo-mcp/internal/format"
)

const (
	defaultDimension         = 512
	defaultDenoisingStrength = 0.75
	gridStep                 = 8
)

// GenerateRequest is one generation to run. An unset InitImage generates from the prompt alone; a set one transforms
// that image instead. Width and Height left at 0 are filled in from the init image, or with 512 pixels each without
// one.
type GenerateRequest struct {
	InitImage []byte

	Checkpoint             string
	ForgePreset            string
	ForgeAdditionalModules []string

	Prompt         string
	NegativePrompt string

	Sampler   string
	Scheduler string
	Steps     int

	Width    int
	Height   int
	CFGScale float64

	// DenoisingStrength is how much of the init image to change, so it is only read with an InitImage; 0 there means
	// the caller left it out. Without an init image it is the hi-res second pass' own denoising strength, where 0
	// leaves Forge's own setting in place.
	DenoisingStrength float64

	Seed int

	EnableHR          bool
	HRScale           float64
	HRUpscaler        string
	HRSecondPassSteps int
	HRCFGScale        float64
}

func (c *Client) Generate(ctx context.Context, req GenerateRequest) ([][]byte, error) {
	width, height, err := outputSize(req)
	if err != nil {
		return nil, err
	}

	if len(req.InitImage) == 0 {
		return c.txt2img(ctx, txt2imgRequestFor(req, width, height))
	}

	return c.img2img(ctx, img2imgRequestFor(req, width, height))
}

func txt2imgRequestFor(req GenerateRequest, width, height int) txt2imgRequest {
	return txt2imgRequest{
		Checkpoint:             req.Checkpoint,
		ForgePreset:            req.ForgePreset,
		ForgeAdditionalModules: req.ForgeAdditionalModules,

		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Steps:          req.Steps,
		Width:          width,
		Height:         height,
		Seed:           req.Seed,
		CFGScale:       req.CFGScale,
		SamplerName:    req.Sampler,
		Scheduler:      req.Scheduler,

		EnableHR:          req.EnableHR,
		HRScale:           req.HRScale,
		HRUpscaler:        req.HRUpscaler,
		HRSecondPassSteps: req.HRSecondPassSteps,
		DenoisingStrength: req.DenoisingStrength,
		HRCFGScale:        req.HRCFGScale,
	}
}

func img2imgRequestFor(req GenerateRequest, width, height int) img2imgRequest {
	strength := req.DenoisingStrength
	if strength == 0 {
		strength = defaultDenoisingStrength
	}

	return img2imgRequest{
		Checkpoint:             req.Checkpoint,
		ForgePreset:            req.ForgePreset,
		ForgeAdditionalModules: req.ForgeAdditionalModules,

		InitImageData: req.InitImage,

		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Steps:          req.Steps,
		Width:          width,
		Height:         height,
		Seed:           req.Seed,
		CFGScale:       req.CFGScale,
		SamplerName:    req.Sampler,
		Scheduler:      req.Scheduler,

		DenoisingStrength: strength,

		IsDoHR:            req.EnableHR,
		HRScale:           req.HRScale,
		HRUpscaler:        req.HRUpscaler,
		HRSecondPassSteps: req.HRSecondPassSteps,
		HRCFGScale:        req.HRCFGScale,
	}
}

// outputSize fills in whichever of width and height the caller left out: 512 each without an init image, and
// otherwise the init image's size, both of them when neither is set and the missing one from the init image's aspect
// ratio when only one is set. Derived sizes land on the grid Forge encodes and decodes on, so the image that comes
// back is the size the caller was told rather than Forge's silently floored version of it.
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

// gridSize rounds down to the eight pixel grid the diffusion backends encode and decode on, so a size taken from an
// image is one Forge can reproduce exactly. A grid step at the low end keeps a tiny image from rounding away to
// nothing.
func gridSize(value int) int {
	return max(gridStep, value/gridStep*gridStep)
}
