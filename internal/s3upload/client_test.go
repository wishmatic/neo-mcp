package s3upload

import (
	"context"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

func TestNewClientAddressingStyle(t *testing.T) {
	tests := []struct {
		name         string
		usePathStyle bool
		wantHost     string
		wantPath     string
	}{
		{
			name:         "path style",
			usePathStyle: true,
			wantHost:     "garage:3900",
			wantPath:     "/test-bucket/i/mcp/x.png",
		},
		{
			name:         "virtual hosted",
			usePathStyle: false,
			wantHost:     "test-bucket.garage:3900",
			wantPath:     "/i/mcp/x.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Region: "test-region", UsePathStyle: tt.usePathStyle}
			client := c.newClient("http://garage:3900", "write-key", "write-secret")

			req, err := s3.NewPresignClient(client).PresignGetObject(context.Background(), &s3.GetObjectInput{
				Bucket: aws.String("test-bucket"),
				Key:    aws.String("i/mcp/x.png"),
			})
			if err != nil {
				t.Fatalf("PresignGetObject() error: %v", err)
			}

			u, err := url.Parse(req.URL)
			if err != nil {
				t.Fatalf("parse presigned URL %q: %v", req.URL, err)
			}

			if u.Host != tt.wantHost {
				t.Fatalf("host = %q, want %q", u.Host, tt.wantHost)
			}

			if u.Path != tt.wantPath {
				t.Fatalf("path = %q, want %q", u.Path, tt.wantPath)
			}
		})
	}
}

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
