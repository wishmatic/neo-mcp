package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

type Deps struct {
	Log       *zap.Logger
	Forge     *sdwebui.Client
	NovelAI   *novelai.Client
	Uploader  *s3upload.Client
	Shortener *shortener.Client
	Resolver  *resolve.Resolver
	OpenAI    *openai.Client
	Store     *store.Client
	Examples  ExamplesConfig
}

func New(deps Deps) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "neo-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(srv, &handlers{
		log:       deps.Log,
		forge:     deps.Forge,
		novelai:   deps.NovelAI,
		gen:       imagegen.New(deps.Forge, deps.NovelAI),
		publisher: publish.New(deps.Uploader, deps.Shortener, deps.Log),
		resolver:  deps.Resolver,
		openai:    deps.OpenAI,
		store:     deps.Store,
		examples:  deps.Examples,
	})

	return srv, nil
}

func registerTools(srv *mcp.Server, h *handlers) {
	if h.forge != nil || h.novelai != nil {
		registerTxt2Img(srv, h)
		registerImg2Img(srv, h)
	}

	if h.novelai != nil {
		registerAnlas(srv, h)
	}

	if h.forge != nil {
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

	if h.examples.Enabled && h.store != nil && h.publisher.Enabled() {
		registerGetExamples(srv, h)
	}
}
