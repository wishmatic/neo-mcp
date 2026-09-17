package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

func publishImages(
	ctx context.Context,
	log *zap.Logger,
	tool string,
	images [][]byte,
	public bool,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
) (*mcp.CallToolResult, generationOutput, error) {
	if uploader != nil {
		urls := make([]string, 0, len(images))
		for _, data := range images {
			url, err := uploader.UploadImage(ctx, data, public)
			if err != nil {
				log.Error(tool+" upload to s3 failed", zap.Error(err))

				return nil, generationOutput{}, err
			}

			urls = append(urls, shortenURL(ctx, log, tool, url, shortenerClient))
		}

		content := make([]mcp.Content, 0, len(urls))
		for _, url := range urls {
			content = append(content, &mcp.TextContent{Text: url})
		}

		return &mcp.CallToolResult{Content: content}, generationOutput{Count: len(urls), URLs: urls}, nil
	}

	content := make([]mcp.Content, 0, len(images))
	for _, data := range images {
		content = append(content, &mcp.ImageContent{
			Data:     data,
			MIMEType: "image/png",
		})
	}

	return &mcp.CallToolResult{Content: content}, generationOutput{Count: len(images)}, nil
}

func shortenURL(ctx context.Context, log *zap.Logger, tool, url string, shortenerClient *shortener.Client) string {
	if shortenerClient == nil {
		return url
	}

	short, err := shortenerClient.Shorten(ctx, url)
	if err != nil {
		log.Warn(tool+" url shortening failed; returning original url", zap.Error(err))

		return url
	}

	return short
}
