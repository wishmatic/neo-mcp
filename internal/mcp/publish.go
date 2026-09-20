package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/present"
	"go.uber.org/zap"
)

func (h *handlers) publishImages(
	ctx context.Context,
	tool string,
	images [][]byte,
	format imgfmt.Format,
) (*mcp.CallToolResult, generationOutput, error) {
	converted, err := convertImages(images, format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("%s: %w", tool, err)
	}

	urls, err := h.publisher.Images(ctx, tool, converted, format.MediaType())
	if err != nil {
		return nil, generationOutput{}, err
	}

	content, failures := present.StoredImages(images, urls)
	for _, failure := range failures {
		h.log.Warn("inline image encoding failed", zap.Int("image", failure.Index), zap.Error(failure.Err))
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
