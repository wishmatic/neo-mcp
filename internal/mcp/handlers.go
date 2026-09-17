package mcp

import (
	"context"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type handlers struct {
	log       *zap.Logger
	forge     *sdwebui.Client
	novelai   *novelai.Client
	gen       *imagegen.Generator
	publisher *publish.Publisher
	resolver  *resolve.Resolver
	openai    *openai.Client
	store     *store.Client
	examples  ExamplesConfig
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
