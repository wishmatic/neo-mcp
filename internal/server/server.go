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
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	mcpServer "github.com/wishmatic/neo-mcp/internal/mcp"
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

const writeTimeout = 10 * time.Minute

type Server struct {
	cfg    config.Config
	log    *zap.Logger
	store  *store.Client
	router *chi.Mux
	http   *http.Server
}

func New(cfg config.Config, log *zap.Logger) (*Server, error) {
	if cfg.APIKey == "" {
		return nil, auth.ErrNoAPIKey
	}

	if cfg.GaragefrontURL != "" && cfg.GaragefrontUserID == "" {
		return nil, fmt.Errorf("GARAGEFRONT_USER_ID is required when GARAGEFRONT_URL is set")
	}

	if cfg.ExamplesEnabled && cfg.ExamplesMax < 1 {
		return nil, fmt.Errorf("EXAMPLES_MAX must be at least 1 when EXAMPLES_ENABLED is set")
	}

	storeClient, err := store.New(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	log.Info("database opened", zap.String("path", cfg.DBPath))

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

	sdClient := sdwebui.New(cfg.SDURL, cfg.ErrorDetail == "verbose")

	uploader, err := s3upload.New(s3upload.Config{
		Endpoint:          cfg.S3Endpoint,
		PublicEndpoint:    cfg.S3PublicEndpoint,
		Bucket:            cfg.S3Bucket,
		Region:            cfg.S3Region,
		AccessKey:         cfg.S3AccessKey,
		SecretKey:         cfg.S3SecretKey,
		ReadonlyAccessKey: cfg.S3ReadonlyAccessKey,
		ReadonlySecretKey: cfg.S3ReadonlySecretKey,
		UsePathStyle:      cfg.S3UsePathStyle,
		PublicBaseURL:     cfg.GaragefrontURL,
		KeyPrefix:         cfg.GaragefrontPrefix(),
	}, log)
	if err != nil {
		log.Warn("s3 upload disabled", zap.Error(err))
		uploader = nil
	}

	var shortenerClient *shortener.Client
	if cfg.ShortenerAPIURL != "" && cfg.ShortenerAPIKey != "" {
		shortenerClient = shortener.New(cfg.ShortenerAPIURL, cfg.ShortenerAPIKey, cfg.ShortenerExpiry)

		log.Info(
			"url shortener enabled",
			zap.String("api_url", cfg.ShortenerAPIURL),
			zap.Int("expiry_seconds", cfg.ShortenerExpiry),
		)
	}

	var objectStore resolve.ObjectStore
	if uploader != nil {
		objectStore = uploader
	}

	resolver, err := resolve.New(objectStore, cfg.GaragefrontURL)
	if err != nil {
		return nil, fmt.Errorf("build image resolver: %w", err)
	}

	var openaiClient *openai.Client
	if cfg.Img2TxtBaseURL != "" {
		openaiClient, err = openai.New(
			cfg.Img2TxtBaseURL,
			cfg.Img2TxtAPIKey,
			cfg.Img2TxtModel,
			cfg.Img2TxtSystemPrompt,
		)
		if err != nil {
			return nil, fmt.Errorf("build img2txt client: %w", err)
		}

		log.Info("img2txt enabled",
			zap.String("base_url", cfg.Img2TxtBaseURL),
			zap.String("model", cfg.Img2TxtModel),
		)
	}

	var novelaiClient *novelai.Client
	if cfg.NovelAIAPIKey != "" {
		novelaiClient = novelai.New(novelai.DefaultBaseURL, cfg.NovelAIAPIKey, cfg.ErrorDetail == "verbose")

		log.Info("novelai enabled", zap.String("base_url", novelai.DefaultBaseURL))
	}

	if cfg.ExamplesEnabled && uploader == nil {
		log.Warn("examples enabled but S3 upload is not configured; no examples will be saved and get_examples is not " +
			"registered")
	}

	mcpSrv, err := mcpServer.New(mcpServer.Deps{
		Log:       log,
		Generator: imagegen.New(sdClient, novelaiClient),
		Bgkill:    bgkill.New(sdClient),
		Publisher: publish.New(uploader, shortenerClient, log),
		NovelAI:   novelaiClient,
		Resolver:  resolver,
		OpenAI:    openaiClient,
		Store:     storeClient,
		Examples: mcpServer.ExamplesConfig{
			Enabled: cfg.ExamplesEnabled,
			Max:     cfg.ExamplesMax,
		},
	})
	if err != nil {
		_ = storeClient.Close()

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
		store:  storeClient,
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

func (s *Server) Run() error {
	s.log.Info("server listening", zap.String("addr", s.cfg.Addr()))

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return errors.Join(s.http.Shutdown(ctx), s.store.Close())
}
