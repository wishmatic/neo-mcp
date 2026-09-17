package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (h *handlers) publishImages(
	ctx context.Context,
	tool string,
	images [][]byte,
	public bool,
) (*mcp.CallToolResult, generationOutput, error) {
	if !h.publisher.Enabled() {
		content := make([]mcp.Content, 0, len(images))
		for _, data := range images {
			content = append(content, &mcp.ImageContent{
				Data:     data,
				MIMEType: "image/png",
			})
		}

		return &mcp.CallToolResult{Content: content}, generationOutput{Count: len(images)}, nil
	}

	urls, err := h.publisher.Images(ctx, tool, images, public)
	if err != nil {
		return nil, generationOutput{}, err
	}

	content := make([]mcp.Content, 0, len(urls))
	for _, url := range urls {
		content = append(content, &mcp.TextContent{Text: url})
	}

	return &mcp.CallToolResult{Content: content}, generationOutput{Count: len(urls), URLs: urls}, nil
}
