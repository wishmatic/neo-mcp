package mcp

import (
	"context"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type handlers struct {
	log           *zap.Logger
	gen           *imagegen.Generator
	bgkillSvc     *bgkill.Service
	publisher     *publish.Publisher
	novelai       *novelai.Client
	resolver      *resolve.Resolver
	openai        *openai.Client
	store         *store.Client
	shortener     *shortener.Client
	examples      ExamplesConfig
	defaultFormat imgfmt.Format
}

func (h *handlers) outputFormat(name string) (imgfmt.Format, error) {
	if name == "" {
		if h.defaultFormat != "" {
			return h.defaultFormat, nil
		}

		return imgfmt.Default, nil
	}

	return imgfmt.Parse(name)
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
