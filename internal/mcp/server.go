package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

func New(
	log *zap.Logger,
	sdClient *sdwebui.Client,
	novelaiClient *novelai.Client,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
	resolver *resolve.Resolver,
	openaiClient *openai.Client,
) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "neo-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(srv, &handlers{
		log:       log,
		forge:     sdClient,
		novelai:   novelaiClient,
		uploader:  uploader,
		shortener: shortenerClient,
		resolver:  resolver,
		openai:    openaiClient,
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
}
