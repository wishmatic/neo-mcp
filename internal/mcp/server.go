package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

func New(
	log *zap.Logger,
	sdClient *sdwebui.Client,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "neo-mcp",
		Version: "0.1.0",
	}, nil)

	registerTools(srv, log, sdClient, uploader, shortenerClient)

	return srv, nil
}

func registerTools(
	srv *mcp.Server,
	log *zap.Logger,
	sdClient *sdwebui.Client,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
) {
	if sdClient != nil {
		registerTxt2Img(srv, log, sdClient, uploader, shortenerClient)
		registerImg2Img(srv, log, sdClient, uploader, shortenerClient)
	}
}
