package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
)

func (h *handlers) publishImages(
	ctx context.Context,
	tool string,
	images [][]byte,
	public, nsfw bool,
	format imgfmt.Format,
) (*mcp.CallToolResult, generationOutput, error) {
	converted, err := convertImages(images, format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("%s: %w", tool, err)
	}

	mediaType := format.MediaType()

	if !h.publisher.Enabled() {
		content := make([]mcp.Content, 0, len(converted))
		for _, data := range converted {
			content = append(content, &mcp.ImageContent{
				Data:     data,
				MIMEType: mediaType,
			})
		}

		return &mcp.CallToolResult{Content: content}, generationOutput{Count: len(converted)}, nil
	}

	urls, err := h.publisher.Images(ctx, tool, converted, mediaType, public, nsfw)
	if err != nil {
		return nil, generationOutput{}, err
	}

	content := make([]mcp.Content, 0, len(urls))
	for _, url := range urls {
		content = append(content, &mcp.TextContent{Text: url})
	}

	return &mcp.CallToolResult{Content: content}, generationOutput{Count: len(urls), URLs: urls}, nil
}

func convertImages(images [][]byte, format imgfmt.Format) ([][]byte, error) {
	converted := make([][]byte, 0, len(images))

	for _, data := range images {
		out, err := imgfmt.Convert(data, format)
		if err != nil {
			return nil, err
		}

		converted = append(converted, out)
	}

	return converted, nil
}
