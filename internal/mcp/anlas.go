package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

type anlasInput struct{}

type anlasOutput struct {
	Total        int  `json:"total" jsonschema:"total Anlas (subscription plus purchased)"`
	Subscription int  `json:"subscription" jsonschema:"subscription Anlas, which resets when the subscription period ends"`
	Purchased    int  `json:"purchased" jsonschema:"purchased Anlas, which does not expire"`
	UsagePercent *int `json:"usage_percent,omitempty" jsonschema:"remaining percentage of the NovelAI V5 usage limit, when the account reports one"`
}

func registerAnlas(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "anlas",
		Description: "Return the NovelAI Anlas (generation credit) balance for the configured account, including the V5 usage meter when the account reports one.",
	}, h.anlas)
}

func (h *handlers) anlas(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ anlasInput,
) (*mcp.CallToolResult, anlasOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "anlas"))

	balance, err := h.novelai.Anlas(ctx)
	if err != nil {
		h.log.Error("anlas balance failed", zap.Error(err))

		return nil, anlasOutput{}, fmt.Errorf("anlas: %w", err)
	}

	out := anlasOutput{
		Total:        balance.Total,
		Subscription: balance.Subscription,
		Purchased:    balance.Purchased,
		UsagePercent: balance.UsagePercent,
	}

	h.log.Info("anlas balance fetched", zap.Int("total", out.Total))

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: anlasSummary(out)}}}, out, nil
}

func anlasSummary(out anlasOutput) string {
	summary := fmt.Sprintf(
		"Anlas: %d (subscription %d, purchased %d)",
		out.Total, out.Subscription, out.Purchased,
	)

	if out.UsagePercent != nil {
		summary += fmt.Sprintf("; V5 usage %d%%", *out.UsagePercent)
	}

	return summary
}
