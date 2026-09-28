package mcp

import (
	"context"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/generation"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type handlers struct {
	log           *zap.Logger
	gen           *generation.Generator
	forge         *forge.Client
	store         *store.Client
	novelai       *novelai.Client
	resolver      *resolve.Resolver
	defaultFormat format.Format
}

func (h *handlers) outputFormat(name string) (format.Format, error) {
	if name == "" {
		if h.defaultFormat != "" {
			return h.defaultFormat, nil
		}

		return format.Default, nil
	}

	return format.Parse(name)
}

func (h *handlers) generationFailure(ctx context.Context, tool string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		h.log.Warn(tool+" aborted: request context cancelled before completion",
			zap.Error(err),
			zap.String("ctx_err", ctxErr.Error()),
		)
	} else {
		h.log.Error(tool+" generation failed", zap.Error(err))
	}

	return fmt.Errorf("%s: %w", tool, err)
}
