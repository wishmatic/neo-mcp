package mcp

import (
	"context"
	"fmt"
	"image/color"
	"math/rand/v2"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/bg"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

type bgInput struct {
	formatInput

	Hex string `json:"hex" jsonschema:"base colour of the background as a hex RGB value, with or without a leading #, for example #4a6fa5"`

	ImageURL string `json:"image_url" jsonschema:"URL of the image to place on the background; the service downloads it (following redirects), and the background takes its size"`
}

func registerBg(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "bg",
		Description: "Place an image on a subtle gradient background built from a single hex colour. The background takes " +
			"the input image's size, and its direction is random on every call. The result is opaque, so the image's " +
			"transparent areas show the background through, which suits a cut-out from bgkill or crop. Downloads the " +
			"input image from a URL (following redirects). Runs locally, so it needs no generation backend.",
		InputSchema: bgSchema(h.defaultFormat),
		Annotations: imageGenerationAnnotations(),
	}, h.bg)
}

func (h *handlers) bg(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in bgInput,
) (*mcp.CallToolResult, generationOutput, error) {
	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("bg: %w", err)
	}

	base, err := bg.ParseHex(in.Hex)
	if err != nil {
		return nil, generationOutput{}, err
	}

	h.log.Debug("tool called",
		zap.String("tool", "bg"),
		zap.String("format", format.String()),
		zap.String("hex", in.Hex),
		zap.String("image_url", in.ImageURL),
	)

	image, err := h.resolver.Fetch(ctx, in.ImageURL)
	if err != nil {
		h.log.Error("bg failed to fetch image",
			zap.String("image_url", in.ImageURL),
			zap.Error(err),
		)

		return nil, generationOutput{}, fmt.Errorf("bg: fetch image: %w", err)
	}

	out, err := bgImage(image, base, format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("bg: %w", err)
	}

	return h.publishImages(ctx, "bg", [][]byte{out}, format)
}

func bgImage(data []byte, base color.NRGBA, format imgfmt.Format) ([]byte, error) {
	src, err := imgfmt.Decode(data)
	if err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	return imgfmt.Encode(bg.Compose(src, base, bg.RandomAngle(rng), rng), format)
}

func bgSchema(def imgfmt.Format) *jsonschema.Schema {
	s, err := jsonschema.For[bgInput](nil)
	if err != nil {
		panic(fmt.Sprintf("bg: infer input schema: %v", err))
	}

	setFormatSchema(s, def)

	return s
}
