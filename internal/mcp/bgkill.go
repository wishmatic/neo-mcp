package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/crop"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"go.uber.org/zap"
)

const defaultSquarePadding = 32

type bgkillInput struct {
	ModelName string `json:"model_name" jsonschema:"BiRefNet model to load"`

	ImageURL string `json:"image_url" jsonschema:"URL of the image to remove the background from; the service downloads it (following redirects)"`

	IsFullMode bool `json:"full_mode,omitempty" jsonschema:"run the model in fp32 instead of fp16; slower and uses more VRAM"`

	IsCrop bool `json:"crop,omitempty" jsonschema:"crop the output to the bounding box of the foreground"`

	IsSquare bool `json:"square,omitempty" jsonschema:"make the cropped output square by centering the foreground on a transparent canvas; implies crop"`

	Padding *int `json:"padding,omitempty" jsonschema:"transparent padding in pixels added around the cropped foreground; defaults to 32 when the output is square"`

	publishInput
}

func registerBgkill(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bgkill",
		Description: "Remove the background from an image via the BiRefNet extension. Downloads the input image from a URL (following redirects), then returns the foreground with a transparent background.",
		InputSchema: bgkillSchema(),
	}, h.bgkill)
}

func (h *handlers) bgkill(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in bgkillInput,
) (*mcp.CallToolResult, generationOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "bgkill"),
		zap.String("model_name", in.ModelName),
		zap.String("image_url", in.ImageURL),
		zap.Bool("full_mode", in.IsFullMode),
		zap.Bool("crop", in.IsCrop),
		zap.Bool("square", in.IsSquare),
		zap.Bool("public", in.Public),
	)

	image, err := h.resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("bgkill failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("bgkill: fetch image: %w", err)
	}

	h.log.Info("bgkill removing background",
		zap.String("model_name", in.ModelName),
		zap.Bool("full_mode", in.IsFullMode),
	)

	out, err := h.forge.Bgkill(ctx, sdwebui.BgkillRequest{
		ModelName:  in.ModelName,
		ImageData:  image,
		IsFullMode: in.IsFullMode,
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			h.log.Warn("bgkill aborted: request context cancelled before completion",
				zap.Error(err),
				zap.String("ctx_err", ctxErr.Error()),
			)
		} else {
			h.log.Error("bgkill failed", zap.Error(err))
		}

		return nil, generationOutput{}, fmt.Errorf("bgkill: %w", err)
	}

	if opts, ok := bgkillCropOptions(in); ok {
		out, err = crop.ToContent(out, opts)
		if err != nil {
			h.log.Error("bgkill failed to crop foreground", zap.Error(err))

			return nil, generationOutput{}, fmt.Errorf("bgkill: %w", err)
		}
	}

	return publishImages(ctx, h.log, "bgkill", [][]byte{out}, in.Public, h.uploader, h.shortener)
}

func bgkillCropOptions(in bgkillInput) (crop.Options, bool) {
	if !in.IsCrop && !in.IsSquare {
		return crop.Options{}, false
	}

	var padding int
	switch {
	case in.Padding != nil:
		padding = *in.Padding
	case in.IsSquare:
		padding = defaultSquarePadding
	}

	return crop.Options{IsSquare: in.IsSquare, Padding: padding}, true
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
	setDefault(s.Properties, "crop", false)
	setDefault(s.Properties, "square", false)
	setDefault(s.Properties, "public", false)

	return s
}
