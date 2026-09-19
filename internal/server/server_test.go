package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wishmatic/neo-mcp/internal/config"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()

	return config.Config{
		APIKey:     "server-key",
		SDURL:      "http://127.0.0.1:7860",
		DBPath:     filepath.Join(t.TempDir(), "neo.db"),
		PublicHost: "https://neo.example.com",
		FilesDir:   filepath.Join(t.TempDir(), "files"),
	}
}

func TestNewWithNovelAIKey(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)
	cfg.NovelAIAPIKey = "sk-test"

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

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

	if _, err := srv.store.RandomExamples(context.Background(), "m", 1); err == nil {
		t.Error("RandomExamples() after Shutdown() error = nil, want a closed store error")
	}
}

func TestNewRequiresPublicHost(t *testing.T) {
	tests := []struct {
		name       string
		publicHost string
	}{
		{name: "missing", publicHost: ""},
		{name: "no scheme", publicHost: "neo.example.com"},
		{name: "with path", publicHost: "https://neo.example.com/neo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.PublicHost = tt.publicHost

			_, err := New(cfg, zap.NewNop())
			if err == nil {
				t.Fatal("New() error = nil, want an error")
			}

			if !strings.Contains(err.Error(), "PUBLIC_HOST") {
				t.Errorf("error = %q, want it to name PUBLIC_HOST", err.Error())
			}
		})
	}
}

func TestNewRequiresFilesDir(t *testing.T) {
	cfg := testConfig(t)
	cfg.FilesDir = ""

	_, err := New(cfg, zap.NewNop())
	if err == nil {
		t.Fatal("New() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "FILES_DIR") {
		t.Errorf("error = %q, want it to name FILES_DIR", err.Error())
	}
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

func TestNewLogsLocalFilesWarning(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	var (
		enabled bool
		warned  bool
	)

	for _, entry := range logs.All() {
		if entry.Message == "local files enabled" && entry.ContextMap()["dir"] == cfg.FilesDir {
			enabled = true
		}

		if entry.Level == zapcore.WarnLevel && strings.Contains(entry.Message, "readable by anyone") {
			warned = true
		}
	}

	if !enabled {
		t.Error("no \"local files enabled\" log entry with the configured directory")
	}

	if !warned {
		t.Error("no warning that stored files are readable by anyone with the URL")
	}
}

func TestServesStoredFile(t *testing.T) {
	cfg := testConfig(t)

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	url, err := srv.files.UploadFile(context.Background(), []byte("png-bytes"), "image/png", false)
	if err != nil {
		t.Fatalf("UploadFile() error: %v", err)
	}

	if !strings.HasPrefix(url, cfg.PublicHost+"/i/") {
		t.Fatalf("url = %q, want a %s/i/ prefix", url, cfg.PublicHost)
	}

	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if rec.Body.String() != "png-bytes" {
		t.Errorf("body = %q, want png-bytes", rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}

func TestRunMaintenanceReturnsOnCancel(t *testing.T) {
	cfg := testConfig(t)

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		srv.RunMaintenance(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunMaintenance did not return after its context was cancelled")
	}
}
