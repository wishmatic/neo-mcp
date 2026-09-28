package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/edit"
	"github.com/wishmatic/neo-mcp/internal/format"
	"go.uber.org/zap"
)

type editInput struct {
	formatInput

	ImageURL string `json:"image_url" jsonschema:"URL of the image to edit; the service downloads it (following redirects)"`

	IsSquare bool `json:"square,omitempty" jsonschema:"make the output square by centering the trimmed content on a transparent canvas"`

	IsCircle bool `json:"circle,omitempty" jsonschema:"cut the output into the circle inscribed in the square canvas; implies square"`

	Padding *int `json:"padding,omitempty" jsonschema:"transparent padding in pixels added around the trimmed content; defaults to 32 for a square and to none otherwise"`
}

func registerEdit(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "edit",
		Description: "Edit an image locally: trim it to the bounding box of its visible content, then optionally pad it, " +
			"center it on a transparent square, or cut it into a circle. Downloads the input image from a URL (following " +
			"redirects). An opaque image has no transparent margins, so trimming leaves it unchanged while the square, " +
			"circle, and padding options still apply.",
		InputSchema: editSchema(h.defaultFormat),
		Annotations: imageGenerationAnnotations(),
	}, h.edit)
}

func (h *handlers) edit(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in editInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("edit: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "edit"),
		zap.String("format", format.String()),
		zap.String("image_url", in.ImageURL),
		zap.Bool("square", in.IsSquare),
		zap.Bool("circle", in.IsCircle),
	)

	source, err := h.resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("edit failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("edit: fetch image: %w", err)
	}

	out, err := editImage(source, editOptions(in), format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("edit: %w", err)
	}

	return h.publishImages(ctx, "edit", [][]byte{out}, format)
}

func editOptions(in editInput) edit.Options {
	opts, _ := edit.OptionsFor(edit.Request{
		IsCrop:   true,
		IsSquare: in.IsSquare,
		IsCircle: in.IsCircle,
		Padding:  in.Padding,
	})

	return opts
}

func editImage(data []byte, opts edit.Options, target format.Format) ([]byte, error) {
	src, err := format.Decode(data)
	if err != nil {
		return nil, err
	}

	out, err := edit.Apply(src, opts)
	if err != nil {
		return nil, err
	}

	return format.Encode(out, target)
}

func editSchema(def format.Format) *jsonschema.Schema {
	s, err := jsonschema.For[editInput](nil)
	if err != nil {
		panic(fmt.Sprintf("edit: infer input schema: %v", err))
	}

	setDefault(s.Properties, "square", false)
	setDefault(s.Properties, "circle", false)
	setFormatSchema(s, def)

	return s
}
