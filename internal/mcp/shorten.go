package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/utils"
	"go.uber.org/zap"
)

type shortenInput struct {
	URL string `json:"url" jsonschema:"the absolute http(s) URL to shorten"`
}

type shortenOutput struct {
	ShortURL string `json:"short_url" jsonschema:"the shortened URL"`
}

func registerShorten(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "shorten",
		Description: "Shorten an arbitrary http(s) URL and return the short link. Use this whenever a shorter link is " +
			"wanted for a URL you already have, not only for links returned by the image tools.",
		InputSchema: shortenSchema(),
	}, h.shorten)
}

func (h *handlers) shorten(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in shortenInput,
) (*mcp.CallToolResult, shortenOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "shorten"),
		zap.String("url", in.URL),
	)

	if h.shortener == nil {
		return nil, shortenOutput{}, fmt.Errorf("shorten: URL shortening is not configured")
	}

	if !utils.IsHTTP(in.URL) {
		return nil, shortenOutput{}, fmt.Errorf("shorten: url must be an absolute http(s) URL")
	}

	short, err := h.shortener.Shorten(ctx, in.URL)
	if err != nil {
		h.log.Error("shorten failed",
			zap.String("url", in.URL),
			zap.Error(err),
		)

		return nil, shortenOutput{}, fmt.Errorf("shorten: %w", err)
	}

	h.log.Info("shorten finished", zap.String("short_url", short))

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: short}}}, shortenOutput{ShortURL: short}, nil
}

func shortenSchema() *jsonschema.Schema {
	s, err := jsonschema.For[shortenInput](nil)
	if err != nil {
		panic(fmt.Sprintf("shorten: infer input schema: %v", err))
	}

	return s
}
