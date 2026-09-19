package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

func (h *handlers) publishImages(
	ctx context.Context,
	tool string,
	images [][]byte,
	nsfw bool,
	format imgfmt.Format,
	mode returnMode,
) (*mcp.CallToolResult, generationOutput, error) {
	converted, err := convertImages(images, format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("%s: %w", tool, err)
	}

	urls, err := h.publisher.Images(ctx, tool, converted, format.MediaType(), nsfw)
	if err != nil {
		return nil, generationOutput{}, err
	}

	return &mcp.CallToolResult{Content: h.imageContent(images, urls, mode)}, generationOutput{Count: len(urls), URLs: urls}, nil
}

func (h *handlers) imageContent(images [][]byte, urls []string, mode returnMode) []mcp.Content {
	if mode == returnImage {
		return h.inlineContent(images, urls)
	}

	content := make([]mcp.Content, 0, len(urls))
	for _, url := range urls {
		content = append(content, &mcp.TextContent{Text: url})
	}

	return content
}

func (h *handlers) inlineContent(images [][]byte, urls []string) []mcp.Content {
	content := []mcp.Content{&mcp.TextContent{Text: inlineCaption(urls)}}

	for i, image := range images {
		inlined, err := imgfmt.Inline(image, imgfmt.InlineMaxEdge, imgfmt.InlineMaxBytes)
		if err != nil {
			h.log.Warn("inline image encoding failed", zap.Int("image", i+1), zap.Error(err))

			content = append(content, &mcp.TextContent{Text: fmt.Sprintf("Image %d could not be attached inline: %v", i+1, err)})

			continue
		}

		content = append(content, &mcp.ImageContent{Data: inlined.Data, MIMEType: inlined.MediaType})
	}

	return content
}

func inlineCaption(urls []string) string {
	caption := fmt.Sprintf("Returned %d image(s) inline for a vision-capable model; inline images are very token-expensive.", len(urls))
	if len(urls) == 0 {
		return caption
	}

	return caption + " Stored URLs: " + strings.Join(urls, " ")
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
