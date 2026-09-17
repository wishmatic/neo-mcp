package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"go.uber.org/zap"
)

const defaultImg2TxtPrompt = "Describe this image in detail."

type img2txtInput struct {
	Image        string  `json:"image" jsonschema:"image to recognise: an http(s) URL, a base64 data URI, or raw base64 PNG, JPEG, or WebP data"`
	Prompt       string  `json:"prompt,omitempty" jsonschema:"what to ask about the image"`
	SystemPrompt string  `json:"system_prompt,omitempty" jsonschema:"system instruction steering the model's response"`
	Model        string  `json:"model,omitempty" jsonschema:"vision model to use; defaults to the server-configured model"`
	Temperature  float64 `json:"temperature,omitempty" jsonschema:"sampling temperature"`
	MaxTokens    int     `json:"max_tokens,omitempty" jsonschema:"maximum tokens in the response"`
	TopP         float64 `json:"top_p,omitempty" jsonschema:"nucleus sampling probability mass"`
	Detail       string  `json:"detail,omitempty" jsonschema:"image detail hint"`
}

type img2txtOutput struct {
	Text  string `json:"text" jsonschema:"the model's textual response"`
	Model string `json:"model" jsonschema:"the model that produced the response"`
}

func registerImg2Txt(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "img2txt",
		Description: "Recognise and analyse an image, returning a textual response. Accepts an http(s) URL (including shortened or Garagefront URLs), a base64 data URI, or raw base64 PNG, JPEG, or WebP data.",
		InputSchema: img2txtSchema(),
	}, h.img2txt)
}

func (h *handlers) img2txt(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in img2txtInput,
) (*mcp.CallToolResult, img2txtOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "img2txt"),
		zap.String("model", in.Model),
		zap.String("prompt", in.Prompt),
		zap.String("detail", in.Detail),
		zap.Float64("temperature", in.Temperature),
		zap.Int("max_tokens", in.MaxTokens),
		zap.Float64("top_p", in.TopP),
	)

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
		zap.String("model", in.Model),
		zap.String("media_type", image.MediaType),
		zap.Int("image_bytes", len(image.Data)),
	)

	result, err := h.openai.Describe(ctx, openai.DescribeRequest{
		ImageData:    image.Data,
		MediaType:    image.MediaType,
		Prompt:       in.Prompt,
		SystemPrompt: in.SystemPrompt,
		Model:        in.Model,
		Temperature:  in.Temperature,
		MaxTokens:    in.MaxTokens,
		TopP:         in.TopP,
		Detail:       in.Detail,
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

	return img2txtOutput{Text: result.Text, Model: result.Model}, nil
}

func img2txtSchema() *jsonschema.Schema {
	s, err := jsonschema.For[img2txtInput](nil)
	if err != nil {
		panic(fmt.Sprintf("img2txt: infer input schema: %v", err))
	}

	setDefault(s.Properties, "prompt", defaultImg2TxtPrompt)
	setDefault(s.Properties, "temperature", 0.2)
	setDefault(s.Properties, "max_tokens", 1024)
	setDefault(s.Properties, "top_p", 1.0)
	setDefault(s.Properties, "detail", "auto")
	s.Properties["detail"].Enum = []any{"auto", "low", "high"}

	return s
}
