package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

const (
	roleUser      mcp.Role = "user"
	roleAssistant mcp.Role = "assistant"
)

func (h *handlers) publishImages(
	ctx context.Context,
	tool string,
	images [][]byte,
	nsfw bool,
	format imgfmt.Format,
	forAssistant bool,
) (*mcp.CallToolResult, generationOutput, error) {
	converted, err := convertImages(images, format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("%s: %w", tool, err)
	}

	urls, err := h.publisher.Images(ctx, tool, converted, format.MediaType(), nsfw)
	if err != nil {
		return nil, generationOutput{}, err
	}

	return &mcp.CallToolResult{Content: h.imageContent(images, urls, forAssistant)}, generationOutput{Count: len(urls), URLs: urls}, nil
}

func (h *handlers) imageContent(images [][]byte, urls []string, forAssistant bool) []mcp.Content {
	content := make([]mcp.Content, 0, 2*len(urls))

	for i, url := range urls {
		content = append(content, &mcp.TextContent{Text: url})

		if i >= len(images) {
			continue
		}

		inlined, err := imgfmt.Inline(images[i], imgfmt.InlineMaxEdge, imgfmt.InlineMaxBytes)
		if err != nil {
			h.log.Warn("inline image encoding failed", zap.Int("image", i+1), zap.Error(err))

			content = append(content, &mcp.TextContent{Text: fmt.Sprintf("Image %d could not be attached inline: %v", i+1, err)})

			continue
		}

		content = append(content, &mcp.ImageContent{
			Data:        inlined.Data,
			MIMEType:    inlined.MediaType,
			Annotations: imageAudience(forAssistant),
		})
	}

	return content
}

// imageAudience scopes an image block to the user, and to the assistant too when the call asked for it. A block whose
// audience is only the user is shown in the client but never reaches the model.
func imageAudience(forAssistant bool) *mcp.Annotations {
	if forAssistant {
		return &mcp.Annotations{Audience: []mcp.Role{roleAssistant, roleUser}}
	}

	return &mcp.Annotations{Audience: []mcp.Role{roleUser}}
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
