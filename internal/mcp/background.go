package mcp

import (
	"context"
	"fmt"
	"image/color"
	"math/rand/v2"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/background"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

type backgroundInput struct {
	formatInput

	Hex string `json:"hex" jsonschema:"base colour of the background as a hex RGB value, with or without a leading #, for example #4a6fa5"`

	ImageURL string `json:"image_url" jsonschema:"URL of the image to place on the background; the service downloads it (following redirects), and the background takes its size"`
}

func registerBackground(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "background",
		Description: "Place an image on a subtle gradient background built from a single hex colour. The background takes " +
			"the input image's size, and its direction is random on every call. The result is opaque, so the image's " +
			"transparent areas show the background through, which suits a cut-out from bgkill or crop. Downloads the " +
			"input image from a URL (following redirects). Runs locally, so it needs no generation backend.",
		InputSchema: backgroundSchema(h.defaultFormat),
		Annotations: imageGenerationAnnotations(),
	}, h.background)
}

func (h *handlers) background(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in backgroundInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("background: %w", err)
	}

	base, err := background.ParseHex(in.Hex)
	if err != nil {
		return nil, generationOutput{}, err
	}

	h.log.Debug("tool called",
		zap.String("tool", "background"),
		zap.String("format", format.String()),
		zap.String("hex", in.Hex),
		zap.String("image_url", in.ImageURL),
	)

	image, err := h.resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("background failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("background: fetch image: %w", err)
	}

	out, err := backgroundImage(image, base, format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("background: %w", err)
	}

	return h.publishImages(ctx, "background", [][]byte{out}, format)
}

func backgroundImage(data []byte, base color.NRGBA, format imgfmt.Format) ([]byte, error) {
	src, err := imgfmt.Decode(data)
	if err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	return imgfmt.Encode(background.Compose(src, base, background.RandomAngle(rng), rng), format)
}

func backgroundSchema(def imgfmt.Format) *jsonschema.Schema {
	s, err := jsonschema.For[backgroundInput](nil)
	if err != nil {
		panic(fmt.Sprintf("background: infer input schema: %v", err))
	}

	setFormatSchema(s, def)

	return s
}
