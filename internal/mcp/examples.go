package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/present"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

const defaultExampleCount = 2

type ExamplesConfig struct {
	Enabled bool
	Max     int
}

type examplesInput struct {
	Model string `json:"model" jsonschema:"model to fetch saved examples for: a Forge checkpoint filename or a NovelAI model id"`
	N     int    `json:"n,omitempty" jsonschema:"optional: how many examples to return; defaults to 2"`
}

type examplesOutput struct {
	Model string `json:"model" jsonschema:"the model the examples are for"`
	Count int    `json:"count" jsonschema:"number of examples returned"`
}

func registerExamples(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "examples",
		Description: "Return randomly chosen examples of previous generations for a model: the query that produced them " +
			"and the URL of the resulting image. Pass `n` to choose how many to return; it defaults to 2.",
		InputSchema: examplesSchema(),
	}, h.examples)
}

func (h *handlers) examples(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in examplesInput,
) (*mcp.CallToolResult, examplesOutput, error) {
	count := in.N
	if count == 0 {
		count = defaultExampleCount
	}

	h.log.Debug("tool called", zap.String("tool", "examples"), zap.String("model", in.Model), zap.Int("n", count))

	examples, err := h.store.RandomExamples(ctx, in.Model, count)
	if err != nil {
		h.log.Error("examples failed", zap.String("model", in.Model), zap.Error(err))

		return nil, examplesOutput{}, fmt.Errorf("examples: %w", err)
	}

	out := examplesOutput{Model: in.Model, Count: len(examples)}

	h.log.Info("examples fetched", zap.String("model", out.Model), zap.Int("count", out.Count))

	text := present.Examples(in.Model, examples)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

// Failures are swallowed: the generation already succeeded and the caller already holds the images.
func (h *handlers) saveExamples(ctx context.Context, tool, model string, query any, urls []string) {
	if !h.examplesConfig.Enabled || h.store == nil || len(urls) == 0 {
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

	meta := store.ExampleMeta{Model: model, Tool: tool, Query: string(raw)}

	if err := h.store.SaveExamples(ctx, meta, urls, h.examplesConfig.Max); err != nil {
		h.log.Warn("examples: saving failed",
			zap.String("tool", tool),
			zap.String("model", model),
			zap.Error(err),
		)
	}
}

func examplesSchema() *jsonschema.Schema {
	s, err := jsonschema.For[examplesInput](nil)
	if err != nil {
		panic(fmt.Sprintf("examples: infer input schema: %v", err))
	}

	s.Properties["n"].Minimum = jsonschema.Ptr(float64(1))
	setDefault(s.Properties, "n", defaultExampleCount)

	return s
}
