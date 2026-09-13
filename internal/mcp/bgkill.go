package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

type bgkillInput struct {
	ModelName string `json:"model_name" jsonschema:"BiRefNet model to load"`

	ImageURL string `json:"image_url" jsonschema:"URL of the image to remove the background from; the service downloads it (following redirects)"`

	FullMode bool `json:"full_mode,omitempty" jsonschema:"run the model in fp32 instead of fp16; slower and uses more VRAM"`
}

func registerBgkill(
	srv *mcp.Server,
	log *zap.Logger,
	client *sdwebui.Client,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bgkill",
		Description: "Remove the background from an image via the BiRefNet extension. Downloads the input image from a URL (following redirects), then returns the foreground with a transparent background.",
		InputSchema: bgkillSchema(),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		in bgkillInput,
	) (*mcp.CallToolResult, generationOutput, error) {
		log.Debug("tool called",
			zap.String("tool", "bgkill"),
			zap.String("model_name", in.ModelName),
			zap.String("image_url", in.ImageURL),
			zap.Bool("full_mode", in.FullMode),
		)

		image, err := client.FetchImage(ctx, in.ImageURL)
		if err != nil {
			log.Error("bgkill failed to fetch image",
				zap.String("image_url", in.ImageURL),
				zap.Error(err),
			)

			return nil, generationOutput{}, fmt.Errorf("bgkill: fetch image: %w", err)
		}

		log.Info("bgkill removing background",
			zap.String("model_name", in.ModelName),
			zap.Bool("full_mode", in.FullMode),
		)

		out, err := client.Bgkill(ctx, sdwebui.BgkillRequest{
			ModelName: in.ModelName,
			ImageData: image,
			FullMode:  in.FullMode,
		})
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				log.Warn("bgkill aborted: request context cancelled before completion",
					zap.Error(err),
					zap.String("ctx_err", ctxErr.Error()),
				)
			} else {
				log.Error("bgkill failed", zap.Error(err))
			}

			return nil, generationOutput{}, fmt.Errorf("bgkill: %w", err)
		}

		return publishImages(ctx, log, "bgkill", [][]byte{out}, uploader, shortenerClient)
	})
}

func bgkillSchema() *jsonschema.Schema {
	s, err := jsonschema.For[bgkillInput](nil)
	if err != nil {
		panic(fmt.Sprintf("bgkill: infer input schema: %v", err))
	}

	models := make([]any, 0, len(sdwebui.BgkillModels))
	for _, model := range sdwebui.BgkillModels {
		models = append(models, model)
	}

	s.Properties["model_name"].Enum = models
	setDefault(s.Properties, "full_mode", false)

	return s
}
