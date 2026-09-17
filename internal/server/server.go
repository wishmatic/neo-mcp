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
	"github.com/wishmatic/neo-mcp/internal/config"
	mcpServer "github.com/wishmatic/neo-mcp/internal/mcp"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

const writeTimeout = 10 * time.Minute

type Server struct {
	cfg    config.Config
	log    *zap.Logger
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

	var store resolve.ObjectStore
	if uploader != nil {
		store = uploader
	}

	resolver, err := resolve.New(store, cfg.GaragefrontURL)
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

	mcpSrv, err := mcpServer.New(log, sdClient, novelaiClient, uploader, shortenerClient, resolver, openaiClient)
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
	return s.http.Shutdown(ctx)
}
