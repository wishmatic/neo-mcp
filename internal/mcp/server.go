package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/diffusion"
	"github.com/wishmatic/neo-mcp/internal/format"
)

func New(clients Clients) (*mcp.Server, error) {
	if clients.Generator == nil {
		clients.Generator = diffusion.New(nil, nil)
	}

	if clients.DefaultOutputFormat == "" {
		clients.DefaultOutputFormat = format.Default
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "neo-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(srv, &clients)

	return srv, nil
}

func registerTools(srv *mcp.Server, c *Clients) {
	if c.Generator.ForgeEnabled() || c.Generator.NovelAIEnabled() {
		registerTxt2Img(srv, c)
		registerImg2Img(srv, c)
	}

	registerEdit(srv, c)
	registerConvert(srv, c)

	if c.Generator.ForgeEnabled() {
		registerBgkill(srv, c)
	}
}
