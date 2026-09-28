package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/diffusion"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type Clients struct {
	Log *zap.Logger

	Generator *diffusion.Client
	Store     *store.Client
	Resolver  *resolve.Client

	DefaultOutputFormat format.Format
}

func New(clients Clients) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "neo-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(srv, buildHandlers(clients))

	return srv, nil
}

func buildHandlers(clients Clients) *handlers {
	h := &handlers{
		log:           clients.Log,
		gen:           clients.Generator,
		store:         clients.Store,
		resolver:      clients.Resolver,
		defaultFormat: clients.DefaultOutputFormat,
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

	registerEdit(srv, h)
	registerConvert(srv, h)

	if h.gen.ForgeEnabled() {
		registerBgkill(srv, h)
	}
}
