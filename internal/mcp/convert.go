package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"go.uber.org/zap"
)

type convertInput struct {
	ImageURL string `json:"image_url" jsonschema:"URL of the image to convert; the service downloads it (following redirects)"`

	Format string `json:"format" jsonschema:"format to convert to: png, jpeg, jxl (JPEG XL), or webp. Ask for a format the image is not already in: a call whose format matches the input's is an error rather than a no-op"`
}

func registerConvert(srv *mcp.Server, c *Clients) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "convert",
		Description: "Re-encode an image as png, jpeg, jxl, or webp, changing nothing else about it. Transparency is kept " +
			"for every format except jpeg, which composites onto white. Downloads the input image from a URL (following " +
			"redirects). Asking for the format the image is already in is an error rather than a no-op, so the call always " +
			"either re-encodes or fails. Runs locally, so it needs no generation backend.",
		InputSchema: convertSchema(),
		Annotations: imageGenerationAnnotations(),
	}, c.convert)
}

func (c *Clients) convert(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in convertInput,
) (*mcp.CallToolResult, generationOutput, error) {
	target, err := format.Parse(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("convert: %w", err)
	}

	c.Log.Debug("tool called",
		zap.String("tool", "convert"),
		zap.String("format", target.String()),
		zap.String("image_url", in.ImageURL),
	)

	source, err := c.Resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		c.Log.Error("convert failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("convert: fetch image: %w", err)
	}

	out, err := format.Reformat(source, target)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("convert: %w", err)
	}

	return c.publishImages(ctx, "convert", [][]byte{out}, target)
}

func convertSchema() *jsonschema.Schema {
	s, err := jsonschema.For[convertInput](nil)
	if err != nil {
		panic(fmt.Sprintf("convert: infer input schema: %v", err))
	}

	setFormatEnum(s)

	return s
}
