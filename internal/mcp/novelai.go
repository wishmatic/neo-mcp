package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"go.uber.org/zap"
)

type novelaiInput struct {
	generationInput

	Model string `json:"model" jsonschema:"NovelAI model id to generate with, e.g. nai-diffusion-5-full"`

	Noise float64 `json:"noise,omitempty" jsonschema:"extra image noise; only used with an init image"`
}

func registerNovelAI(srv *mcp.Server, c *Clients) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "novelai",
		Description: "Generate an image via NovelAI, synchronously: the call blocks until generation completes and " +
			"returns the image. Pass init_image_url to transform an existing image instead of generating from " +
			"scratch; the service downloads it (following redirects). Without an init image, width and height default " +
			"to 512. With one, they default to the init image's own size, and when only one of them is given, the " +
			"other keeps the init image's aspect ratio. Calls may spend Anlas on your NovelAI account.",
		InputSchema: novelaiSchema(c.DefaultOutputFormat),
		Annotations: imageGenerationAnnotations(),
	}, c.novelai)
}

func (c *Clients) novelai(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in novelaiInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := c.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("novelai: %w", err)
	}

	c.Log.Debug("tool called",
		zap.String("tool", "novelai"),
		zap.String("format", format.String()),
		zap.Int("inline_max_edge", in.InlineMaxEdge),
		zap.Int("inline_max_bytes", in.InlineMaxBytes),
		zap.String("model", in.Model),
		zap.String("init_image_url", in.InitImageURL),
		zap.String("sampler", in.SamplingMethod),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
		zap.Float64("cfg_scale", in.CFGScale),
		zap.Int("seed", in.Seed),
		zap.Float64("denoising_strength", in.DenoisingStrength),
		zap.Float64("noise", in.Noise),
	)

	initImage, err := c.initImage(ctx, "novelai", in.InitImageURL)
	if err != nil {
		return nil, generationOutput{}, err
	}

	c.Log.Info("novelai generating synchronously",
		zap.String("model", in.Model),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
	)

	images, err := c.NovelAI.Generate(ctx, novelaiGenerateRequest(in, initImage))
	if err != nil {
		return nil, generationOutput{}, c.generationFailure(ctx, "novelai", err)
	}

	c.Log.Info("novelai generation finished", zap.Int("images", len(images)))

	result, out, err := c.publishImages(ctx, "novelai", images, format, in.inlineBudget())
	if err != nil {
		return nil, generationOutput{}, err
	}

	return result, out, nil
}

func novelaiGenerateRequest(in novelaiInput, initImage []byte) novelai.GenerateRequest {
	return novelai.GenerateRequest{
		InitImage: initImage,

		Model:          in.Model,
		Prompt:         in.Prompt,
		NegativePrompt: in.NegativePrompt,
		Sampler:        in.SamplingMethod,
		Steps:          in.SamplingSteps,

		Width:  in.Width,
		Height: in.Height,
		Scale:  in.CFGScale,

		Strength: in.DenoisingStrength,
		Noise:    in.Noise,

		Seed: in.Seed,
	}
}

func novelaiSchema(def format.Format) *jsonschema.Schema {
	s, err := jsonschema.For[novelaiInput](nil)
	if err != nil {
		panic(fmt.Sprintf("novelai: infer input schema: %v", err))
	}

	setGenerationDefaults(s)
	setDefault(s.Properties, "sampler_name", novelai.DefaultSampler)
	setDefault(s.Properties, "noise", 0.0)
	setFormatSchema(s, def)

	s.Properties["sampler_name"].Description = "the NovelAI sampler to use; defaults to " + novelai.DefaultSampler
	s.Properties["width"].Description = "output width in pixels; defaults to 512 without an init image, or to the init " +
		"image's width, or to the width that keeps its aspect ratio when only height is set; NovelAI rounds it up to a " +
		"multiple of 64"
	s.Properties["height"].Description = "output height in pixels; defaults to 512 without an init image, or to the " +
		"init image's height, or to the height that keeps its aspect ratio when only width is set; NovelAI rounds it up " +
		"to a multiple of 64"
	s.Properties["init_image_url"].Description = "URL of an image to transform instead of generating from scratch; " +
		"the service downloads it (following redirects)"
	s.Properties["denoising_strength"].Description = "how much of the init image to change, where higher changes " +
		"more and 1 ignores it; defaults to 0.75"

	return s
}
