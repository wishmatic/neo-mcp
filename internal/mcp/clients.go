package mcp

import (
	"context"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type Clients struct {
	Log *zap.Logger

	Forge    *forge.Client
	NovelAI  *novelai.Client
	OpenAI   *openai.Client
	Store    *store.Client
	Resolver *resolve.Client

	DefaultOutputFormat format.Format
}

func (c *Clients) outputFormat(name string) (format.Format, error) {
	if name == "" {
		if c.DefaultOutputFormat != "" {
			return c.DefaultOutputFormat, nil
		}

		return format.Default, nil
	}

	return format.Parse(name)
}

func (c *Clients) generationFailure(ctx context.Context, tool string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		c.Log.Warn(tool+" aborted: request context cancelled before completion",
			zap.Error(err),
			zap.String("ctx_err", ctxErr.Error()),
		)
	} else {
		c.Log.Error(tool+" generation failed", zap.Error(err))
	}

	return fmt.Errorf("%s: %w", tool, err)
}
