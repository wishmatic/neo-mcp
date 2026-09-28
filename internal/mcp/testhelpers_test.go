package mcp

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/store"
)

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
