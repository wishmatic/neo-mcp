package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/generation"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"go.uber.org/zap"
)

type Deps struct {
	Log       *zap.Logger
	Generator *generation.Generator
	Forge     *forge.Client
	Publisher *publish.Publisher
	NovelAI   *novelai.Client
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
		forge:         deps.Forge,
		publisher:     deps.Publisher,
		novelai:       deps.NovelAI,
		resolver:      deps.Resolver,
		defaultFormat: deps.OutputFormat,
	}

	if h.gen == nil {
		h.gen = generation.New(nil, nil)
	}

	if h.publisher == nil {
		h.publisher = publish.New(nil, deps.Log)
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

	if h.novelai != nil {
		registerAnlas(srv, h)
	}

	registerCrop(srv, h)
	registerConvert(srv, h)

	if h.forge != nil {
		registerBgkill(srv, h)
	}
}
