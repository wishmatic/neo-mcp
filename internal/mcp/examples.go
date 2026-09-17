package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

const examplesPerCall = 2

type ExamplesConfig struct {
	Enabled bool
	Max     int
}

type getExamplesInput struct {
	Model string `json:"model" jsonschema:"model to fetch saved examples for: a Forge checkpoint filename or a NovelAI model id"`
}

type getExamplesOutput struct {
	Model string `json:"model" jsonschema:"the model the examples are for"`
	Count int    `json:"count" jsonschema:"number of examples returned"`
}

func registerGetExamples(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_examples",
		Description: "Return up to two randomly chosen examples of previous generations for a model: the query that " +
			"produced them and the URL of the resulting image.",
		InputSchema: getExamplesSchema(),
	}, h.getExamples)
}

func (h *handlers) getExamples(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in getExamplesInput,
) (*mcp.CallToolResult, getExamplesOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "get_examples"), zap.String("model", in.Model))

	examples, err := h.store.RandomExamples(ctx, in.Model, examplesPerCall)
	if err != nil {
		h.log.Error("get_examples failed", zap.String("model", in.Model), zap.Error(err))

		return nil, getExamplesOutput{}, fmt.Errorf("get_examples: %w", err)
	}

	out := getExamplesOutput{Model: in.Model, Count: len(examples)}

	h.log.Info("examples fetched", zap.String("model", out.Model), zap.Int("count", out.Count))

	text := examplesMarkdown(in.Model, examples)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

// Failures are swallowed: the generation already succeeded and the caller already holds the images.
func (h *handlers) saveExamples(ctx context.Context, tool, model string, query any, urls []string) {
	if !h.examples.Enabled || h.store == nil || len(urls) == 0 {
		return
	}

	raw, err := json.Marshal(query)
	if err != nil {
		h.log.Warn("examples: marshal query failed",
			zap.String("tool", tool),
			zap.String("model", model),
			zap.Error(err),
		)

		return
	}

	for _, url := range urls {
		meta := store.ExampleMeta{Model: model, Tool: tool, Query: string(raw), URL: url}

		if _, err := h.store.SaveExample(ctx, meta, h.examples.Max); err != nil {
			h.log.Warn("examples: saving failed",
				zap.String("tool", tool),
				zap.String("model", model),
				zap.Error(err),
			)
		}
	}
}

func getExamplesSchema() *jsonschema.Schema {
	s, err := jsonschema.For[getExamplesInput](nil)
	if err != nil {
		panic(fmt.Sprintf("get_examples: infer input schema: %v", err))
	}

	return s
}
