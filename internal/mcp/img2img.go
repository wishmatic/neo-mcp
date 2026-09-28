package mcp

import (
	"context"
	"fmt"
	"math"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/generation"
	"go.uber.org/zap"
)

type img2imgInput struct {
	generationInput

	InitImageURL string `json:"init_image_url" jsonschema:"URL of the image to transform; the service downloads it (following redirects)"`

	DenoisingStrength float64 `json:"denoising_strength,omitempty" jsonschema:"how much to change the input image (0 keeps it identical, 1 ignores it)"`

	Noise float64 `json:"noise,omitempty" jsonschema:"NovelAI only: extra image noise; ignored by Forge"`
}

func registerImg2Img(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "img2img",
		Description: "Transform an existing image, via the local Stable Diffusion WebUI (Forge Neo) instance or " +
			"via NovelAI when the model is a NovelAI model id. Downloads the input image from a URL (following redirects), " +
			"then blocks until generation completes and returns the image. Width and height default to the init image's " +
			"own size, rounded to the model's pixel grid; when only one of them is given, the other keeps the init image's " +
			"aspect ratio. Hi-res (HR) second-pass upscaling is Forge only: " +
			"NovelAI ignores the hr_ fields and returns the requested size. When HR upscaling is enabled, denoising_strength " +
			"is required.",
		InputSchema: img2imgSchema(h.defaultFormat),
		Annotations: imageGenerationAnnotations(),
	}, h.img2img)
}

func (h *handlers) img2img(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in img2imgInput,
) (*mcp.CallToolResult, generationOutput, error) {
	provider := generation.ProviderOf(in.Model)

	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("img2img: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "img2img"),
		zap.String("provider", provider),
		zap.String("format", format.String()),
		zap.String("model", in.Model),
		zap.String("forge_preset", in.ForgePreset),
		zap.Strings("vae_and_text_models", in.VAEAndTextModels),
		zap.String("init_image_url", in.InitImageURL),
		zap.String("sampler", in.SamplingMethod),
		zap.String("scheduler", in.ScheduleType),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
		zap.Float64("cfg_scale", in.CFGScale),
		zap.Float64("denoising_strength", in.DenoisingStrength),
		zap.Float64("noise", in.Noise),
		zap.Int("seed", in.Seed),
		zap.Bool("enable_hr", in.EnableHR),
		zap.Float64("hr_scale", in.HRScale),
		zap.String("hr_upscaler", in.HRUpscaler),
		zap.Int("hr_second_pass_steps", in.HRSecondPassSteps),
		zap.Float64("hr_cfg", in.HRCFGScale),
	)

	initImage, err := h.resolver.Fetch(ctx, in.InitImageURL)
	if err != nil {
		h.log.Error("img2img failed to fetch init image",
			zap.String("init_image_url", in.InitImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("img2img: fetch init image: %w", err)
	}

	in.Width, in.Height, err = initImageSize(in.generationInput, initImage)
	if err != nil {
		h.log.Error("img2img failed to read the init image size",
			zap.String("init_image_url", in.InitImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("img2img: read init image size: %w", err)
	}

	h.log.Info("img2img generating synchronously",
		zap.String("provider", provider),
		zap.String("model", in.Model),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
	)

	images, err := h.gen.Img2Img(ctx, generationImg2ImgRequest(in, initImage))
	if err != nil {
		return nil, generationOutput{}, h.generationFailure(ctx, "img2img", err)
	}

	h.log.Info("img2img generation finished", zap.Int("images", len(images)))

	result, out, err := h.publishImages(ctx, "img2img", images, format)
	if err != nil {
		return nil, generationOutput{}, err
	}

	return result, out, nil
}

func img2imgSchema(def format.Format) *jsonschema.Schema {
	s, err := jsonschema.For[img2imgInput](nil)
	if err != nil {
		panic(fmt.Sprintf("img2img: infer input schema: %v", err))
	}

	setDefault(s.Properties, "negative_prompt", "")
	setDefault(s.Properties, "steps", 20)
	setDefault(s.Properties, "seed", -1)
	setDefault(s.Properties, "cfg_scale", 7.0)
	setDefault(s.Properties, "denoising_strength", 0.75)
	setDefault(s.Properties, "noise", 0.0)
	setDefault(s.Properties, "sampler_name", "")
	setDefault(s.Properties, "scheduler", "")
	setFormatSchema(s, def)

	s.Properties["width"].Description = "output width in pixels; defaults to the init image's width, or to the width " +
		"that keeps the init image's aspect ratio when only height is set"
	s.Properties["height"].Description = "output height in pixels; defaults to the init image's height, or to the " +
		"height that keeps the init image's aspect ratio when only width is set"

	return s
}

// initImageSize fills in whichever of width and height the caller left out from the init image: both of them when
// neither is set, and the missing one from the init image's aspect ratio when only one is set. Derived sizes land on
// the sampling grid, so the image that comes back is the size the caller was told rather than the backends' silently
// floored version of it.
func initImageSize(in generationInput, initImage []byte) (int, int, error) {
	if in.Width != 0 && in.Height != 0 {
		return in.Width, in.Height, nil
	}

	width, height, err := format.Dimensions(initImage)
	if err != nil {
		return 0, 0, fmt.Errorf("%w (pass width and height to override)", err)
	}

	switch {
	case in.Width == 0 && in.Height == 0:
		return samplingGridSize(width), samplingGridSize(height), nil
	case in.Width == 0:
		return samplingGridSize(aspectDimension(in.Height, width, height)), in.Height, nil
	default:
		return in.Width, samplingGridSize(aspectDimension(in.Width, height, width)), nil
	}
}

func aspectDimension(known, otherSource, knownSource int) int {
	return int(math.Round(float64(known) * float64(otherSource) / float64(knownSource)))
}

// samplingGridSize rounds down to the eight pixel grid the diffusion backends encode and decode on, so a size taken
// from an image is one the backend can reproduce exactly. A grid step at the low end keeps a tiny image from rounding
// away to nothing.
func samplingGridSize(value int) int {
	const gridStep = 8

	return max(gridStep, value/gridStep*gridStep)
}
