package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
)

func New(clients Clients) (*mcp.Server, error) {
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
	if c.Forge != nil {
		registerForge(srv, c)
		registerBgkill(srv, c)
	}

	if c.NovelAI != nil {
		registerNovelAI(srv, c)
	}

	if c.OpenAI != nil {
		registerOpenAI(srv, c)
	}

	registerEdit(srv, c)
	registerConvert(srv, c)
}
