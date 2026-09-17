package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewWithNovelAIKey(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	srv, err := New(config.Config{
		APIKey:        "server-key",
		SDURL:         "http://127.0.0.1:7860",
		NovelAIAPIKey: "sk-test",
		NovelAIURL:    "https://image.novelai.net",
	}, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}

	enabled := false

	for _, entry := range logs.All() {
		if entry.Message == "novelai enabled" {
			enabled = true
		}

		context := fmt.Sprint(entry.ContextMap())
		if strings.Contains(entry.Message, "sk-test") || strings.Contains(context, "sk-test") {
			t.Errorf("log entry %q leaks the API key", entry.Message)
		}
	}

	if !enabled {
		t.Error("no \"novelai enabled\" log entry, want one")
	}
}
