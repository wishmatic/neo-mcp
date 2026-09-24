package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

type bgkillInput struct {
	formatInput

	ModelName string `json:"model_name" jsonschema:"BiRefNet model to load"`

	ImageURL string `json:"image_url" jsonschema:"URL of the image to remove the background from; the service downloads it (following redirects)"`

	IsFullMode bool `json:"full_mode,omitempty" jsonschema:"run the model in fp32 instead of fp16; slower and uses more VRAM"`
}

func registerBgkill(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "bgkill",
		Description: "Remove the background from an image via the BiRefNet extension, returning the foreground with a transparent background. Downloads the input image from a URL (following redirects). Use the crop tool to trim or circularise the result.",
		InputSchema: bgkillSchema(h.defaultFormat),
		Annotations: imageGenerationAnnotations(),
	}, h.bgkill)
}

func (h *handlers) bgkill(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in bgkillInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("bgkill: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "bgkill"),
		zap.String("format", format.String()),
		zap.String("model_name", in.ModelName),
		zap.String("image_url", in.ImageURL),
		zap.Bool("full_mode", in.IsFullMode),
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

	out, err := h.bgkillSvc.Remove(ctx, bgkillRequest(in, image))
	if err != nil {
		return nil, generationOutput{}, h.generationFailure(ctx, "bgkill", err)
	}

	return h.publishImages(ctx, "bgkill", [][]byte{out}, format)
}

func bgkillRequest(in bgkillInput, image []byte) bgkill.Request {
	return bgkill.Request{
		ModelName:  in.ModelName,
		ImageData:  image,
		IsFullMode: in.IsFullMode,
	}
}

func bgkillSchema(def imgfmt.Format) *jsonschema.Schema {
	s, err := jsonschema.For[bgkillInput](nil)
	if err != nil {
		panic(fmt.Sprintf("bgkill: infer input schema: %v", err))
	}

	models := make([]any, 0, len(bgkill.Models))
	for _, model := range bgkill.Models {
		models = append(models, model)
	}

	s.Properties["model_name"].Enum = models
	setDefault(s.Properties, "full_mode", false)
	setFormatSchema(s, def)

	return s
}
