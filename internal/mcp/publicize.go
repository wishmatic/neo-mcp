package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

type publicizeInput struct {
	ImageURL string `json:"image_url" jsonschema:"URL of the image to publish; the service downloads it (following redirects)"`
}

type publicizeOutput struct {
	URL string `json:"url" jsonschema:"the world-readable URL for the published image"`
}

func registerPublicize(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "publicize",
		Description: "Publish an image to the world-readable public namespace and return a URL anyone can open. " +
			"Downloads the input image from a URL (following redirects), then stores it unchanged. Only use this when " +
			"the user has explicitly asked for a publicly viewable image.",
		InputSchema: publicizeSchema(),
	}, h.publicize)
}

func (h *handlers) publicize(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in publicizeInput,
) (*mcp.CallToolResult, publicizeOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "publicize"),
		zap.String("image_url", in.ImageURL),
	)

	if h.uploader == nil {
		return nil, publicizeOutput{}, fmt.Errorf("publicize: S3 upload is not configured")
	}

	image, err := h.resolver.Resolve(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("publicize failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, publicizeOutput{}, fmt.Errorf("publicize: fetch image: %w", err)
	}

	h.log.Info("publicize uploading image",
		zap.String("media_type", image.MediaType),
		zap.Int("image_bytes", len(image.Data)),
	)

	url, err := h.uploader.UploadFile(ctx, image.Data, image.MediaType, true)
	if err != nil {
		h.log.Error("publicize upload to s3 failed", zap.Error(err))

		return nil, publicizeOutput{}, fmt.Errorf("publicize: %w", err)
	}

	url = shortenURL(ctx, h.log, "publicize", url, h.shortener)

	h.log.Info("publicize finished", zap.String("url", url))

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: url}}}, publicizeOutput{URL: url}, nil
}

func publicizeSchema() *jsonschema.Schema {
	s, err := jsonschema.For[publicizeInput](nil)
	if err != nil {
		panic(fmt.Sprintf("publicize: infer input schema: %v", err))
	}

	return s
}
