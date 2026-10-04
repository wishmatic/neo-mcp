package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"go.uber.org/zap"
)

type openaiInput struct {
	Provider string `json:"provider,omitempty" jsonschema:"name of the configured endpoint to call; defaults to the first one configured"`

	Model  string `json:"model,omitempty" jsonschema:"image model id, such as flux-schnell or gpt-image-1; the names change, so a rejection means the endpoint's own model list is worth checking. Unset uses the endpoint's own default"`
	Prompt string `json:"prompt" jsonschema:"the text prompt describing the image to generate"`

	N    *int   `json:"n,omitempty" jsonschema:"how many images to generate, each of them billed; unset uses the endpoint's own default of 1"`
	Size string `json:"size,omitempty" jsonschema:"requested output size or a model-specific resolution, such as 1024x1024 or auto; what a model accepts varies"`

	// Inputs are read by this service and travel with the request inline, so the endpoint needs to reach nothing.
	Image  string   `json:"image,omitempty" jsonschema:"an image to transform instead of generating from scratch; a URL the service downloads (following redirects), a base64 data URI, or raw base64"`
	Images []string `json:"images,omitempty" jsonschema:"input images for the models that take more than one, each read the way image is"`
	Mask   string   `json:"mask,omitempty" jsonschema:"an inpainting mask, read the way image is; white marks the regions to generate"`

	Seed              *int     `json:"seed,omitempty" jsonschema:"random seed, on the models that take one; it may improve reproducibility without guaranteeing it"`
	Strength          *float64 `json:"strength,omitempty" jsonschema:"how far the output may move from the input image, from 0 to 1, where lower stays closer to it"`
	GuidanceScale     *float64 `json:"guidance_scale,omitempty" jsonschema:"how closely the model follows the prompt, from 0 to 20"`
	NumInferenceSteps *int     `json:"num_inference_steps,omitempty" jsonschema:"denoising steps, from 1 to 100, where more is slower and usually better"`

	formatInput
}

type openaiOutput struct {
	generationOutput

	Provider string  `json:"provider" jsonschema:"name of the endpoint the images came from"`
	Model    string  `json:"model" jsonschema:"the model that was asked for, empty when the endpoint's own default was used"`
	CostUSD  float64 `json:"costUsd" jsonschema:"what the endpoint billed for the call, which is 0 when it reports no cost"`
}

func registerOpenAI(srv *mcp.Server, c *Clients) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "openai",
		Description: "Generate one or more images through an OpenAI-compatible endpoint, synchronously: the call " +
			"blocks until generation completes and returns the images. provider names which of the configured " +
			"endpoints to call, defaulting to the first. Pass image to transform an existing image instead of " +
			"generating from scratch, and mask to confine the change to the regions it marks. Bills the endpoint's " +
			"balance for each image; the charge is reported in costUsd where the endpoint reports one.",
		InputSchema: openaiSchema(c.DefaultOutputFormat, c.OpenAI.ProviderNames()),
		Annotations: imageGenerationAnnotations(),
	}, c.openai)
}

func (c *Clients) openai(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in openaiInput,
) (*mcp.CallToolResult, openaiOutput, error) {
	format, err := c.outputFormat(in.Format)
	if err != nil {
		return nil, openaiOutput{}, fmt.Errorf("openai: %w", err)
	}

	c.Log.Debug("tool called",
		zap.String("tool", "openai"),
		zap.String("provider", in.Provider),
		zap.String("format", format.String()),
		zap.Int("inline_max_edge", in.InlineMaxEdge),
		zap.Int("inline_max_bytes", in.InlineMaxBytes),
		zap.String("model", in.Model),
		zap.Int("input_images", len(in.Images)),
		zap.Bool("has_image", in.Image != ""),
		zap.Bool("has_mask", in.Mask != ""),
	)

	request, err := c.openaiRequest(ctx, in)
	if err != nil {
		return nil, openaiOutput{}, err
	}

	result, err := c.OpenAI.GenerateImages(ctx, in.Provider, request)
	if err != nil {
		return nil, openaiOutput{}, c.generationFailure(ctx, "openai", err)
	}

	images, err := c.generatedImages(ctx, result.Images)
	if err != nil {
		return nil, openaiOutput{}, err
	}

	c.Log.Info("openai generation finished",
		zap.String("provider", result.Provider),
		zap.Int("images", len(images)),
		zap.Float64("cost_usd", result.CostUSD),
	)

	callResult, out, err := c.publishImages(ctx, "openai", images, format, in.inlineBudget())
	if err != nil {
		return nil, openaiOutput{}, err
	}

	return callResult, openaiOutput{
		generationOutput: out,
		Provider:         result.Provider,
		Model:            in.Model,
		CostUSD:          result.CostUSD,
	}, nil
}

func openaiSchema(def format.Format, providers []string) *jsonschema.Schema {
	s, err := jsonschema.For[openaiInput](nil)
	if err != nil {
		panic(fmt.Sprintf("openai: infer input schema: %v", err))
	}

	setFormatSchema(s, def)
	setProviderSchema(s, providers)

	return s
}

func setProviderSchema(s *jsonschema.Schema, providers []string) {
	if len(providers) == 0 {
		return
	}

	names := make([]any, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider)
	}

	s.Properties["provider"].Enum = names

	setDefault(s.Properties, "provider", providers[0])
}
