package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/format"
	"go.uber.org/zap"
)

const (
	forgeDefaultSampler   = "DPM++ 2M"
	forgeDefaultScheduler = "Automatic"
)

type forgeInput struct {
	generationInput

	Model       string `json:"model" jsonschema:"checkpoint (model) filename to load"`
	ForgePreset string `json:"forge_preset" jsonschema:"Forge UI preset to apply"`

	VAEAndTextModels []string `json:"vae_and_text_models,omitempty" jsonschema:"VAE and text encoder model filenames to load. Leave empty to use the defaults bundled with the checkpoint"`

	ScheduleType string `json:"scheduler,omitempty"`

	EnableHR          bool    `json:"enable_hr,omitempty" jsonschema:"enable hi-res (HR) second-pass upscaling"`
	HRScale           float64 `json:"hr_scale,omitempty" jsonschema:"if HR is enabled, the hi-res upscaling factor (e.g. 2 for 2x)"`
	HRUpscaler        string  `json:"hr_upscaler,omitempty" jsonschema:"if HR is enabled, the hi-res upscaler to use. Leave empty to disable upscaling"`
	HRSecondPassSteps int     `json:"hr_second_pass_steps,omitempty" jsonschema:"if HR is enabled, the number of steps for the hi-res second pass"`
	HRCFGScale        float64 `json:"hr_cfg,omitempty" jsonschema:"if HR is enabled, the CFG scale for the hi-res second pass"`
}

func registerForge(srv *mcp.Server, c *Clients) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "forge",
		Description: "Generate an image with the local Stable Diffusion WebUI (Forge Neo) instance, synchronously: " +
			"the call blocks until generation completes and returns the image. Pass init_image_url to transform an " +
			"existing image instead of generating from scratch; the service downloads it (following redirects). " +
			"Without an init image, width and height default to 512. With one, they default to the init image's own " +
			"size on the model's pixel grid, and when only one of them is given, the other keeps the init image's " +
			"aspect ratio. Hi-res (HR) second-pass upscaling runs after generation. When HR upscaling is enabled, " +
			"denoising_strength is required.",
		InputSchema: forgeSchema(c.DefaultOutputFormat),
		Annotations: imageGenerationAnnotations(),
	}, c.forge)
}

func (c *Clients) forge(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in forgeInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := c.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("forge: %w", err)
	}

	c.Log.Debug("tool called",
		zap.String("tool", "forge"),
		zap.String("format", format.String()),
		zap.Int("inline_max_edge", in.InlineMaxEdge),
		zap.Int("inline_max_bytes", in.InlineMaxBytes),
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
		zap.Int("seed", in.Seed),
		zap.Bool("enable_hr", in.EnableHR),
		zap.Float64("hr_scale", in.HRScale),
		zap.String("hr_upscaler", in.HRUpscaler),
		zap.Int("hr_second_pass_steps", in.HRSecondPassSteps),
		zap.Float64("denoising_strength", in.DenoisingStrength),
		zap.Float64("hr_cfg", in.HRCFGScale),
	)

	initImage, err := c.initImage(ctx, "forge", in.InitImageURL)
	if err != nil {
		return nil, generationOutput{}, err
	}

	c.Log.Info("forge generating synchronously",
		zap.String("model", in.Model),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
	)

	images, err := c.Forge.Generate(ctx, forgeGenerateRequest(in, initImage))
	if err != nil {
		return nil, generationOutput{}, c.generationFailure(ctx, "forge", err)
	}

	c.Log.Info("forge generation finished", zap.Int("images", len(images)))

	result, out, err := c.publishImages(ctx, "forge", images, format, in.inlineBudget())
	if err != nil {
		return nil, generationOutput{}, err
	}

	return result, out, nil
}

func forgeGenerateRequest(in forgeInput, initImage []byte) forge.GenerateRequest {
	return forge.GenerateRequest{
		InitImage: initImage,

		Checkpoint:             in.Model,
		ForgePreset:            in.ForgePreset,
		ForgeAdditionalModules: in.VAEAndTextModels,

		Prompt:         in.Prompt,
		NegativePrompt: in.NegativePrompt,

		Sampler:   in.SamplingMethod,
		Scheduler: in.ScheduleType,
		Steps:     in.SamplingSteps,

		Width:    in.Width,
		Height:   in.Height,
		CFGScale: in.CFGScale,

		DenoisingStrength: in.DenoisingStrength,

		Seed: in.Seed,

		EnableHR:          in.EnableHR,
		HRScale:           in.HRScale,
		HRUpscaler:        in.HRUpscaler,
		HRSecondPassSteps: in.HRSecondPassSteps,
		HRCFGScale:        in.HRCFGScale,
	}
}

func forgeSchema(def format.Format) *jsonschema.Schema {
	s, err := jsonschema.For[forgeInput](nil)
	if err != nil {
		panic(fmt.Sprintf("forge: infer input schema: %v", err))
	}

	setGenerationDefaults(s)
	setDefault(s.Properties, "sampler_name", forgeDefaultSampler)
	setDefault(s.Properties, "scheduler", forgeDefaultScheduler)
	setFormatSchema(s, def)

	s.Properties["sampler_name"].Description = "the sampler to use; defaults to " + forgeDefaultSampler
	s.Properties["scheduler"].Description = "the scheduler to use; defaults to " + forgeDefaultScheduler
	s.Properties["width"].Description = "output width in pixels; defaults to 512 without an init image, or to the init " +
		"image's width, or to the width that keeps its aspect ratio when only height is set"
	s.Properties["height"].Description = "output height in pixels; defaults to 512 without an init image, or to the " +
		"init image's height, or to the height that keeps its aspect ratio when only width is set"
	s.Properties["init_image_url"].Description = "URL of an image to transform instead of generating from scratch; " +
		"the service downloads it (following redirects)"
	s.Properties["denoising_strength"].Description = "how much of the init image to change, where higher changes " +
		"more and 1 ignores it; defaults to 0.75. Without an init image, the denoising strength for the hi-res " +
		"second pass, which is required when HR upscaling is enabled"

	return s
}
