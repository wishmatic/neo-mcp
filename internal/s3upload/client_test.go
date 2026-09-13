package s3upload

import (
	"testing"

	"go.uber.org/zap"
)

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
