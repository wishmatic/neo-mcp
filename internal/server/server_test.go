package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/config"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()

	return config.Config{
		APIKey: "server-key",
		SDURL:  "http://127.0.0.1:7860",
		DBPath: filepath.Join(t.TempDir(), "neo.db"),
	}
}

func withS3(cfg config.Config) config.Config {
	cfg.S3Endpoint = "http://127.0.0.1:3900"
	cfg.S3Bucket = "test-bucket"
	cfg.S3Region = "test-region"
	cfg.S3AccessKey = "write-key"
	cfg.S3SecretKey = "write-secret"
	cfg.S3ReadonlyAccessKey = "read-key"
	cfg.S3ReadonlySecretKey = "read-secret"

	return cfg
}

func TestNewWithNovelAIKey(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)
	cfg.NovelAIAPIKey = "sk-test"

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}

	enabled := false
	baseURL := ""

	for _, entry := range logs.All() {
		if entry.Message == "novelai enabled" {
			enabled = true
			baseURL, _ = entry.ContextMap()["base_url"].(string)
		}

		context := fmt.Sprint(entry.ContextMap())
		if strings.Contains(entry.Message, "sk-test") || strings.Contains(context, "sk-test") {
			t.Errorf("log entry %q leaks the API key", entry.Message)
		}
	}

	if !enabled {
		t.Error("no \"novelai enabled\" log entry, want one")
	}

	if baseURL != novelai.DefaultBaseURL {
		t.Errorf("base_url = %q, want %s", baseURL, novelai.DefaultBaseURL)
	}
}

func TestNewCreatesDatabase(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if _, err := os.Stat(cfg.DBPath); err != nil {
		t.Fatalf("stat database: %v", err)
	}

	if srv.store == nil {
		t.Fatal("srv.store = nil, want an open database")
	}

	opened := false

	for _, entry := range logs.All() {
		if entry.Message == "database opened" && entry.ContextMap()["path"] == cfg.DBPath {
			opened = true
		}
	}

	if !opened {
		t.Error("no \"database opened\" log entry with the configured path")
	}
}

func TestNewFailsOnUnopenableDatabase(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")

	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	cfg := testConfig(t)
	cfg.DBPath = filepath.Join(blocker, "neo.db")

	_, err := New(cfg, zap.NewNop())
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "open database") {
		t.Errorf("error = %q, want it to mention opening the database", err.Error())
	}
}

func TestShutdownClosesStore(t *testing.T) {
	cfg := testConfig(t)

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}

	if _, err := srv.store.AddReview(context.Background(), "m", 5, "ok"); err == nil {
		t.Error("AddReview() after Shutdown() error = nil, want a closed store error")
	}
}

func TestNewRejectsInvalidExamplesMax(t *testing.T) {
	cfg := testConfig(t)
	cfg.ExamplesEnabled = true
	cfg.ExamplesMax = 0

	_, err := New(cfg, zap.NewNop())
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "EXAMPLES_MAX") {
		t.Errorf("error = %q, want it to name EXAMPLES_MAX", err.Error())
	}

	cfg.ExamplesMax = 1

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() with EXAMPLES_MAX=1 error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
}

func TestNewRejectsInvalidOutputFormat(t *testing.T) {
	cfg := testConfig(t)
	cfg.OutputFormat = "nonsense"

	_, err := New(cfg, zap.NewNop())
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "OUTPUT_FORMAT") {
		t.Errorf("error = %q, want it to name OUTPUT_FORMAT", err.Error())
	}

	for _, want := range []string{"png", "jpeg", "jxl", "webp"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	if _, statErr := os.Stat(cfg.DBPath); !os.IsNotExist(statErr) {
		t.Errorf("database was created despite the invalid format: %v", statErr)
	}
}

func TestNewAcceptsOutputFormat(t *testing.T) {
	cfg := testConfig(t)
	cfg.OutputFormat = "JXL"

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
}

func TestNewWarnsWhenExamplesHaveNoUploader(t *testing.T) {
	tests := []struct {
		name     string
		cfg      func(config.Config) config.Config
		wantWarn bool
	}{
		{
			name: "enabled without s3",
			cfg: func(cfg config.Config) config.Config {
				cfg.ExamplesEnabled = true
				cfg.ExamplesMax = 4

				return cfg
			},
			wantWarn: true,
		},
		{
			name: "enabled with s3",
			cfg: func(cfg config.Config) config.Config {
				cfg.ExamplesEnabled = true
				cfg.ExamplesMax = 4

				return withS3(cfg)
			},
		},
		{
			name: "disabled without s3",
			cfg:  func(cfg config.Config) config.Config { return cfg },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)

			srv, err := New(tt.cfg(testConfig(t)), zap.New(core))
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

			warned := false

			for _, entry := range logs.All() {
				if entry.Level == zapcore.WarnLevel && strings.Contains(entry.Message, "examples") {
					warned = true
				}
			}

			if warned != tt.wantWarn {
				t.Errorf("warning logged = %v, want %v", warned, tt.wantWarn)
			}
		})
	}
}
