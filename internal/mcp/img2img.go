package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
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
			"then blocks until generation completes and returns the image.",
		InputSchema: img2imgSchema(h.defaultFormat),
	}, h.img2img)
}

func (h *handlers) img2img(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in img2imgInput,
) (*mcp.CallToolResult, generationOutput, error) {
	provider := imagegen.ProviderOf(in.Model)

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
		zap.Bool("public", in.Public),
	)

	initImage, err := h.resolver.Fetch(ctx, in.InitImageURL)
	if err != nil {
		h.log.Error("img2img failed to fetch init image",
			zap.String("init_image_url", in.InitImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("img2img: fetch init image: %w", err)
	}

	h.log.Info("img2img generating synchronously",
		zap.String("provider", provider),
		zap.String("model", in.Model),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
	)

	images, err := h.gen.Img2Img(ctx, imagegenImg2ImgRequest(in, initImage))
	if err != nil {
		return nil, generationOutput{}, h.generationFailure(ctx, "img2img", err)
	}

	h.log.Info("img2img generation finished", zap.Int("images", len(images)))

	result, out, err := h.publishImages(ctx, "img2img", images, in.Public, format)
	if err != nil {
		return nil, generationOutput{}, err
	}

	h.saveExamples(ctx, "img2img", in.Model, in, out.URLs)

	return result, out, nil
}

func img2imgSchema(def imgfmt.Format) *jsonschema.Schema {
	s, err := jsonschema.For[img2imgInput](nil)
	if err != nil {
		panic(fmt.Sprintf("img2img: infer input schema: %v", err))
	}

	setDefault(s.Properties, "negative_prompt", "")
	setDefault(s.Properties, "steps", 20)
	setDefault(s.Properties, "width", 512)
	setDefault(s.Properties, "height", 512)
	setDefault(s.Properties, "seed", -1)
	setDefault(s.Properties, "cfg_scale", 7.0)
	setDefault(s.Properties, "denoising_strength", 0.75)
	setDefault(s.Properties, "noise", 0.0)
	setDefault(s.Properties, "sampler_name", "")
	setDefault(s.Properties, "scheduler", "")
	setDefault(s.Properties, "public", false)
	setFormatSchema(s, def)

	return s
}
