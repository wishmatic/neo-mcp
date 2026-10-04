package mcp

import (
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func zapNop() *zap.Logger {
	return zap.NewNop()
}

func newTestStore(t *testing.T) *store.Client {
	t.Helper()

	client, _ := newTestStoreAt(t)

	return client
}

func newTestStoreAt(t *testing.T) (*store.Client, string) {
	t.Helper()

	base, err := url.Parse("https://cdn.example.com")
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "files")

	client, err := store.New(store.Config{Dir: dir, PublicBase: base}, zapNop())
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}

	return client, dir
}

func newResolver(t *testing.T) *resolve.Client {
	t.Helper()

	resolver, err := resolve.New(nil, "", nil)
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	return resolver
}

func observedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)

	return zap.New(core), logs
}

func callTool(t *testing.T, srv *mcp.Server, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) error: %v", name, err)
	}

	if result.IsError {
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				t.Fatalf("CallTool(%s) tool error: %s", name, text.Text)
			}
		}

		t.Fatalf("CallTool(%s) tool error: %+v", name, result.Content)
	}

	return result
}

func decodeJSONBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	return decoded
}

func numberField(t *testing.T, body map[string]any, key string) float64 {
	t.Helper()

	value, ok := body[key].(float64)
	if !ok {
		t.Fatalf("%s = %T, want a number", key, body[key])
	}

	return value
}

func singleRequestBody(t *testing.T, log *requestLog) map[string]any {
	t.Helper()

	_, bodies := log.snapshot()
	if len(bodies) != 1 {
		t.Fatalf("requests = %d, want 1", len(bodies))
	}

	return decodeJSONBody(t, bodies[0])
}

func singleRequestPath(t *testing.T, log *requestLog) string {
	t.Helper()

	paths, _ := log.snapshot()
	if len(paths) != 1 {
		t.Fatalf("requests = %d, want 1", len(paths))
	}

	return paths[0]
}
