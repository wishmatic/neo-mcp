package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/diffusion"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type Deps struct {
	Log       *zap.Logger
	Generator *diffusion.Generator
	Store     *store.Client
	Resolver  *resolve.Resolver

	OutputFormat format.Format
}

func New(deps Deps) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "neo-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(srv, buildHandlers(deps))

	return srv, nil
}

func buildHandlers(deps Deps) *handlers {
	h := &handlers{
		log:           deps.Log,
		gen:           deps.Generator,
		store:         deps.Store,
		resolver:      deps.Resolver,
		defaultFormat: deps.OutputFormat,
	}

	if h.gen == nil {
		h.gen = diffusion.New(nil, nil)
	}

	if h.defaultFormat == "" {
		h.defaultFormat = format.Default
	}

	return h
}

func registerTools(srv *mcp.Server, h *handlers) {
	if h.gen.ForgeEnabled() || h.gen.NovelAIEnabled() {
		registerTxt2Img(srv, h)
		registerImg2Img(srv, h)
	}

	registerCrop(srv, h)
	registerConvert(srv, h)

	if h.gen.ForgeEnabled() {
		registerBgkill(srv, h)
	}
}
