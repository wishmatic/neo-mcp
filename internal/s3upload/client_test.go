package s3upload

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewPublicBaseURL(t *testing.T) {
	base := Config{
		Endpoint:      "http://127.0.0.1:3900",
		Bucket:        "test-bucket",
		Region:        "test-region",
		AccessKey:     "write-key",
		SecretKey:     "write-secret",
		PublicBaseURL: "https://cdn.example.com",
		KeyPrefix:     "i/mcp",
	}

	t.Run("readonly credentials not required", func(t *testing.T) {
		if _, err := New(base, zap.NewNop()); err != nil {
			t.Fatalf("New() error: %v", err)
		}
	})

	t.Run("base url without scheme", func(t *testing.T) {
		cfg := base
		cfg.PublicBaseURL = "cdn.example.com"

		if _, err := New(cfg, zap.NewNop()); err == nil {
			t.Fatalf("New() expected error for base URL without scheme")
		}
	})

	t.Run("missing key prefix", func(t *testing.T) {
		cfg := base
		cfg.KeyPrefix = ""

		if _, err := New(cfg, zap.NewNop()); err == nil {
			t.Fatalf("New() expected error for missing key prefix")
		}
	})
}

func TestNewMissingConfig(t *testing.T) {
	base := Config{
		Endpoint:          "http://127.0.0.1:3900",
		Bucket:            "test-bucket",
		Region:            "test-region",
		AccessKey:         "write-key",
		SecretKey:         "write-secret",
		ReadonlyAccessKey: "read-key",
		ReadonlySecretKey: "read-secret",
	}

	fields := map[string]func(*Config){
		"endpoint":            func(c *Config) { c.Endpoint = "" },
		"bucket":              func(c *Config) { c.Bucket = "" },
		"region":              func(c *Config) { c.Region = "" },
		"access key":          func(c *Config) { c.AccessKey = "" },
		"secret key":          func(c *Config) { c.SecretKey = "" },
		"readonly access key": func(c *Config) { c.ReadonlyAccessKey = "" },
		"readonly secret key": func(c *Config) { c.ReadonlySecretKey = "" },
	}

	for name, mutate := range fields {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)

			if _, err := New(cfg, zap.NewNop()); err == nil {
				t.Fatalf("New() with missing %s expected error, got nil", name)
			}
		})
	}
}
