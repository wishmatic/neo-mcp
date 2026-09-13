package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/wishmatic/neo-mcp/internal/auth"
)

type Config struct {
	Host                string `env:"HOST" envDefault:"0.0.0.0"`
	Port                int    `env:"PORT" envDefault:"8080"`
	APIKey              string `env:"API_KEY"`
	AuthModeRaw         string `env:"AUTH_MODE" envDefault:"bearer"`
	LogLevel            string `env:"LOG_LEVEL" envDefault:"info"`
	ErrorDetail         string `env:"ERROR_DETAIL" envDefault:"useful"`
	WriteTimeoutSeconds int    `env:"WRITE_TIMEOUT_SECONDS" envDefault:"600"`

	SDURL string `env:"SD_URL" envDefault:"http://127.0.0.1:7860"`

	S3Endpoint          string `env:"S3_ENDPOINT"`
	S3PublicEndpoint    string `env:"S3_PUBLIC_ENDPOINT"`
	S3Bucket            string `env:"S3_BUCKET"`
	S3Region            string `env:"S3_REGION"`
	S3AccessKey         string `env:"S3_ACCESS_KEY"`
	S3SecretKey         string `env:"S3_SECRET_KEY"`
	S3ReadonlyAccessKey string `env:"S3_READONLY_ACCESS_KEY"`
	S3ReadonlySecretKey string `env:"S3_READONLY_SECRET_KEY"`
	S3PresignExpiry     int    `env:"S3_PRESIGN_EXPIRY" envDefault:"604800"`

	ShortenerAPIURL string `env:"SHORTENER_API_URL"`
	ShortenerAPIKey string `env:"SHORTENER_API_KEY"`
	ShortenerExpiry int    `env:"SHORTENER_EXPIRY_SECONDS" envDefault:"0"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}

	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) AuthMode() auth.AuthMode {
	switch strings.ToLower(strings.TrimSpace(c.AuthModeRaw)) {
	case "basic":
		return auth.AuthBasic
	default:
		return auth.AuthBearer
	}
}
