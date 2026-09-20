package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

type inlineInput struct {
	ImageURL string `json:"image_url" jsonschema:"URL of the image to fetch and return inline; the service downloads it (following redirects)"`
}

type inlineOutput struct {
	URL       string `json:"url" jsonschema:"the URL the image was fetched from"`
	MediaType string `json:"media_type" jsonschema:"media type of the returned image block"`
	Bytes     int    `json:"bytes" jsonschema:"encoded size of the returned image block in bytes"`
}

func registerInline(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "inline",
		Description: "Fetch an image from a URL and return it inline as an MCP image block, downscaled and re-encoded so a " +
			"vision-capable model can see it. Use it to look at an image the user links to or one another tool points at.",
		InputSchema: inlineSchema(),
	}, h.inline)
}

func (h *handlers) inline(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in inlineInput,
) (*mcp.CallToolResult, inlineOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "inline"), zap.String("image_url", in.ImageURL))

	data, err := h.resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("inline fetch failed", zap.String("image_url", in.ImageURL), zap.Error(err))

		return nil, inlineOutput{}, fmt.Errorf("inline: fetch image: %w", err)
	}

	out := inlineOutput{URL: in.ImageURL}

	image, err := imgfmt.Inline(data, imgfmt.InlineMaxEdge, imgfmt.InlineMaxBytes)
	if err != nil {
		h.log.Warn("inline encoding failed", zap.String("image_url", in.ImageURL), zap.Error(err))

		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
			"Could not inline %s: %v", in.ImageURL, err,
		)}}}, out, nil
	}

	out.MediaType = image.MediaType
	out.Bytes = len(image.Data)

	h.log.Info("inline image fetched",
		zap.String("image_url", in.ImageURL),
		zap.String("media_type", out.MediaType),
		zap.Int("bytes", out.Bytes),
	)

	content := []mcp.Content{
		&mcp.TextContent{Text: fmt.Sprintf("Inline image from %s (%s, %d bytes).", in.ImageURL, out.MediaType, out.Bytes)},
		&mcp.ImageContent{Data: image.Data, MIMEType: image.MediaType, Annotations: imageAudience(true)},
	}

	return &mcp.CallToolResult{Content: content}, out, nil
}

func inlineSchema() *jsonschema.Schema {
	s, err := jsonschema.For[inlineInput](nil)
	if err != nil {
		panic(fmt.Sprintf("inline: infer input schema: %v", err))
	}

	return s
}
