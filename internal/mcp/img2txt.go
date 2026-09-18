package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"go.uber.org/zap"
)

type img2txtInput struct {
	Image  string `json:"image" jsonschema:"image to recognise: an http(s) URL, a base64 data URI, or raw base64 PNG, JPEG, or WebP data"`
	Prompt string `json:"prompt,omitempty" jsonschema:"optional question or focus for the description; omit it to use the server-configured default prompt"`
}

type img2txtOutput struct {
	Text      string `json:"text" jsonschema:"the model's exhaustive description of the image"`
	Model     string `json:"model" jsonschema:"the model that produced the description"`
	Truncated bool   `json:"truncated" jsonschema:"true when the description was cut off at the server's output limit and is therefore incomplete"`
}

func registerImg2Txt(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "img2txt",
		Description: "Produce an exhaustive textual description of an image using the server-configured vision model. " +
			"Accepts an http(s) URL (including shortened or Garagefront URLs), a base64 data URI, or raw base64 PNG, " +
			"JPEG, or WebP data. Pass `prompt` to focus the description on a question or detail; omitting it uses the " +
			"server-configured default prompt. Sampling and image detail are fixed server configuration. When the " +
			"response is cut off at the output limit, `truncated` is true and the text is incomplete.",
		InputSchema: img2txtSchema(),
	}, h.img2txt)
}

func (h *handlers) img2txt(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in img2txtInput,
) (*mcp.CallToolResult, img2txtOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "img2txt"))

	out, err := h.runImg2Txt(ctx, in)
	if err != nil {
		return nil, img2txtOutput{}, err
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: out.Text}}}, out, nil
}

func (h *handlers) runImg2Txt(ctx context.Context, in img2txtInput) (img2txtOutput, error) {
	image, err := h.resolver.Resolve(ctx, in.Image)
	if err != nil {
		h.log.Error("img2txt failed to resolve image", zap.Error(err))

		return img2txtOutput{}, fmt.Errorf("img2txt: resolve image: %w", err)
	}

	h.log.Info("img2txt recognising image",
		zap.String("media_type", image.MediaType),
		zap.Int("image_bytes", len(image.Data)),
	)

	result, err := h.openai.Describe(ctx, openai.DescribeRequest{
		ImageData: image.Data,
		MediaType: image.MediaType,
		Prompt:    in.Prompt,
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			h.log.Warn("img2txt aborted: request context cancelled before completion",
				zap.Error(err),
				zap.String("ctx_err", ctxErr.Error()),
			)
		} else {
			h.log.Error("img2txt failed", zap.Error(err))
		}

		return img2txtOutput{}, fmt.Errorf("img2txt: %w", err)
	}

	return img2txtOutput{Text: result.Text, Model: result.Model, Truncated: result.Truncated}, nil
}

func img2txtSchema() *jsonschema.Schema {
	s, err := jsonschema.For[img2txtInput](nil)
	if err != nil {
		panic(fmt.Sprintf("img2txt: infer input schema: %v", err))
	}

	return s
}
