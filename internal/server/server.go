package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/auth"
	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/config"
	"github.com/wishmatic/neo-mcp/internal/filestore"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	mcpServer "github.com/wishmatic/neo-mcp/internal/mcp"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"go.uber.org/zap"
)

const writeTimeout = 10 * time.Minute

type Server struct {
	cfg    config.Config
	log    *zap.Logger
	files  *filestore.Client
	router *chi.Mux
	http   *http.Server
}

func New(cfg config.Config, log *zap.Logger) (*Server, error) {
	if cfg.APIKey == "" {
		return nil, auth.ErrNoAPIKey
	}

	publicBase, err := cfg.PublicBase()
	if err != nil {
		return nil, err
	}

	if publicBase == nil {
		return nil, fmt.Errorf("PUBLIC_HOST is required")
	}

	if cfg.FilesDir == "" {
		return nil, fmt.Errorf("FILES_DIR must not be empty")
	}

	outputFormat, err := outputFormatFrom(cfg)
	if err != nil {
		return nil, err
	}

	files, err := filestore.New(filestore.Config{
		Dir:        cfg.FilesDir,
		PublicBase: publicBase,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("configure file storage: %w", err)
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.ClientIPFromRemoteAddr)
	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	files.Register(router)

	log.Info("local files enabled", zap.String("dir", cfg.FilesDir))
	log.Warn("stored files are readable by anyone with the URL")

	sdClient := sdwebui.New(cfg.SDURL, cfg.ErrorDetail == "verbose")

	resolver, err := resolve.New(files, publicBase.String())
	if err != nil {
		return nil, fmt.Errorf("build image resolver: %w", err)
	}

	var novelaiClient *novelai.Client
	if cfg.NovelAIAPIKey != "" {
		novelaiClient = novelai.New(novelai.DefaultBaseURL, cfg.NovelAIAPIKey, cfg.ErrorDetail == "verbose")

		log.Info("novelai enabled", zap.String("base_url", novelai.DefaultBaseURL))
	}

	mcpSrv, err := mcpServer.New(mcpServer.Deps{
		Log:          log,
		Generator:    imagegen.New(sdClient, novelaiClient),
		Bgkill:       bgkill.New(sdClient),
		Publisher:    publish.New(files, log),
		NovelAI:      novelaiClient,
		Resolver:     resolver,
		OutputFormat: outputFormat,
	})
	if err != nil {
		return nil, fmt.Errorf("build mcp server: %w", err)
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpSrv
	}, nil)

	protected := auth.Middleware(log, cfg.APIKey)(mcpHandler)

	router.Mount("/mcp", protected)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &Server{
		cfg:    cfg,
		log:    log,
		files:  files,
		router: router,
		http: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       60 * time.Second,
		},
	}, nil
}

func outputFormatFrom(cfg config.Config) (imgfmt.Format, error) {
	if cfg.OutputFormat == "" {
		return imgfmt.Default, nil
	}

	format, err := imgfmt.Parse(cfg.OutputFormat)
	if err != nil {
		return "", fmt.Errorf("OUTPUT_FORMAT: %w", err)
	}

	return format, nil
}

func (s *Server) Run() error {
	s.log.Info("server listening", zap.String("addr", s.cfg.Addr()))

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
