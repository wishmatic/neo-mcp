package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type Deps struct {
	Log       *zap.Logger
	Generator *imagegen.Generator
	Bgkill    *bgkill.Service
	Publisher *publish.Publisher
	NovelAI   *novelai.Client
	Resolver  *resolve.Resolver
	OpenAI    *openai.Client
	Store     *store.Client
	Shortener *shortener.Client
	Examples  ExamplesConfig
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
		log:       deps.Log,
		gen:       deps.Generator,
		bgkillSvc: deps.Bgkill,
		publisher: deps.Publisher,
		novelai:   deps.NovelAI,
		resolver:  deps.Resolver,
		openai:    deps.OpenAI,
		store:     deps.Store,
		shortener: deps.Shortener,
		examples:  deps.Examples,
	}

	if h.gen == nil {
		h.gen = imagegen.New(nil, nil)
	}

	if h.bgkillSvc == nil {
		h.bgkillSvc = bgkill.New(nil)
	}

	if h.publisher == nil {
		h.publisher = publish.New(nil, nil, deps.Log)
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

	if h.bgkillSvc.Enabled() {
		registerBgkill(srv, h)
	}

	if h.openai != nil {
		registerImg2Txt(srv, h)
	}

	if h.publisher.Enabled() {
		registerPublicize(srv, h)
	}

	if h.store != nil {
		registerReviews(srv, h)
	}

	if h.shortener != nil {
		registerShorten(srv, h)
	}

	if h.examples.Enabled && h.store != nil && h.publisher.Enabled() {
		registerGetExamples(srv, h)
	}
}
