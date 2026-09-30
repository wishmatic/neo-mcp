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

type bgkillInput struct {
	formatInput

	ModelName string `json:"model_name" jsonschema:"BiRefNet model to load"`

	ImageURL string `json:"image_url" jsonschema:"URL of the image to remove the background from; the service downloads it (following redirects)"`

	IsFullMode bool `json:"full_mode,omitempty" jsonschema:"run the model in fp32 instead of fp16; slower and uses more VRAM"`
}

func registerBgkill(srv *mcp.Server, c *Clients) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bgkill",
		Description: "Remove the background from an image via the BiRefNet extension, returning the foreground with a transparent background. Downloads the input image from a URL (following redirects). Use the edit tool to trim or circularise the result.",
		InputSchema: bgkillSchema(c.DefaultOutputFormat),
		Annotations: imageGenerationAnnotations(),
	}, c.bgkill)
}

func (c *Clients) bgkill(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in bgkillInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := c.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("bgkill: %w", err)
	}

	c.Log.Debug("tool called",
		zap.String("tool", "bgkill"),
		zap.String("format", format.String()),
		zap.String("model_name", in.ModelName),
		zap.String("image_url", in.ImageURL),
		zap.Bool("full_mode", in.IsFullMode),
	)

	image, err := c.Resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		c.Log.Error("bgkill failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("bgkill: fetch image: %w", err)
	}

	c.Log.Info("bgkill removing background",
		zap.String("model_name", in.ModelName),
		zap.Bool("full_mode", in.IsFullMode),
	)

	out, err := c.Forge.Bgkill(ctx, forge.BgkillRequest{
		ModelName:  in.ModelName,
		ImageData:  image,
		IsFullMode: in.IsFullMode,
	})
	if err != nil {
		return nil, generationOutput{}, c.generationFailure(ctx, "bgkill", err)
	}

	return c.publishImages(ctx, "bgkill", [][]byte{out}, format)
}

func bgkillSchema(def format.Format) *jsonschema.Schema {
	s, err := jsonschema.For[bgkillInput](nil)
	if err != nil {
		panic(fmt.Sprintf("bgkill: infer input schema: %v", err))
	}

	models := make([]any, 0, len(forge.BgkillModels))
	for _, model := range forge.BgkillModels {
		models = append(models, model)
	}

	s.Properties["model_name"].Enum = models
	setDefault(s.Properties, "full_mode", false)
	setFormatSchema(s, def)

	return s
}
