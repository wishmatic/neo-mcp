package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/crop"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

type cropInput struct {
	formatInput

	ImageURL string `json:"image_url" jsonschema:"URL of the image to crop; the service downloads it (following redirects)"`

	IsSquare bool `json:"square,omitempty" jsonschema:"make the cropped output square by centering it on a transparent canvas"`

	IsCircle bool `json:"circle,omitempty" jsonschema:"cut the cropped output into the circle inscribed in the square canvas; implies square"`

	Padding *int `json:"padding,omitempty" jsonschema:"transparent padding in pixels added around the cropped content; defaults to 32 for a square and to none otherwise"`
}

func registerCrop(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "crop",
		Description: "Trim an image to the bounding box of its visible content, then optionally pad it, center it on a " +
			"transparent square, or cut it into a circle. Downloads the input image from a URL (following redirects). " +
			"Runs locally, so it needs no generation backend. An opaque image has no transparent margins, so trimming " +
			"leaves it unchanged while the square, circle, and padding options still apply.",
		InputSchema: cropSchema(h.defaultFormat),
		Annotations: imageGenerationAnnotations(),
	}, h.crop)
}

func (h *handlers) crop(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in cropInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("crop: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "crop"),
		zap.String("format", format.String()),
		zap.String("image_url", in.ImageURL),
		zap.Bool("square", in.IsSquare),
		zap.Bool("circle", in.IsCircle),
	)

	source, err := h.resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("crop failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("crop: fetch image: %w", err)
	}

	out, err := cropImage(source, cropOptions(in), format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("crop: %w", err)
	}

	return h.publishImages(ctx, "crop", [][]byte{out}, format)
}

func cropOptions(in cropInput) crop.Options {
	opts, _ := crop.OptionsFor(crop.Request{
		IsCrop:   true,
		IsSquare: in.IsSquare,
		IsCircle: in.IsCircle,
		Padding:  in.Padding,
	})

	return opts
}

func cropImage(data []byte, opts crop.Options, format imgfmt.Format) ([]byte, error) {
	src, err := imgfmt.Decode(data)
	if err != nil {
		return nil, err
	}

	out, err := crop.Apply(src, opts)
	if err != nil {
		return nil, err
	}

	return imgfmt.Encode(out, format)
}

func cropSchema(def imgfmt.Format) *jsonschema.Schema {
	s, err := jsonschema.For[cropInput](nil)
	if err != nil {
		panic(fmt.Sprintf("crop: infer input schema: %v", err))
	}

	setDefault(s.Properties, "square", false)
	setDefault(s.Properties, "circle", false)
	setFormatSchema(s, def)

	return s
}
