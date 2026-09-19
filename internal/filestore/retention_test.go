package filestore

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func backdate(t *testing.T, client *Client, key string, when time.Time) {
	t.Helper()

	path, err := client.safePath(key)
	if err != nil {
		t.Fatalf("safePath() error: %v", err)
	}

	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestSweepKeepsNewFiles(t *testing.T) {
	client, _ := newTestClient(t, 30)
	now := time.Now()

	_, oldKey := upload(t, client, "image/png", false)
	_, newKey := upload(t, client, "image/png", false)

	backdate(t, client, oldKey, now.Add(-60*24*time.Hour))

	deleted, err := client.Sweep(now)
	if err != nil {
		t.Fatalf("Sweep() error: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	if _, err := client.GetObject(context.Background(), oldKey); err == nil {
		t.Error("GetObject(old key) error = nil, want the file to be gone")
	}

	if _, err := client.GetObject(context.Background(), newKey); err != nil {
		t.Errorf("GetObject(new key) error: %v, want the file kept", err)
	}
}

func TestSweepRemovesEmptyMonthDirectories(t *testing.T) {
	client, dir := newTestClient(t, 30)
	now := time.Now()

	_, firstKey := upload(t, client, "image/png", false)
	_, secondKey := upload(t, client, "image/png", false)

	backdate(t, client, firstKey, now.Add(-60*24*time.Hour))
	backdate(t, client, secondKey, now.Add(-60*24*time.Hour))

	path, err := client.safePath(firstKey)
	if err != nil {
		t.Fatalf("safePath() error: %v", err)
	}

	monthDir := filepath.Dir(path)

	deleted, err := client.Sweep(now)
	if err != nil {
		t.Fatalf("Sweep() error: %v", err)
	}

	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	if _, err := os.Stat(monthDir); !os.IsNotExist(err) {
		t.Errorf("stat %s = %v, want the empty month directory removed", monthDir, err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("stat %s: %v, want the storage root kept", dir, err)
	}
}

func TestSweepKeepsUnrelatedDirectories(t *testing.T) {
	client, dir := newTestClient(t, 30)
	now := time.Now()

	other := filepath.Join(dir, "other")
	if err := os.MkdirAll(other, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	otherFile := filepath.Join(other, "x.png")
	if err := os.WriteFile(otherFile, []byte("bytes"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := os.Chtimes(otherFile, now.Add(-60*24*time.Hour), now.Add(-60*24*time.Hour)); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if _, err := client.Sweep(now); err != nil {
		t.Fatalf("Sweep() error: %v", err)
	}

	if _, err := os.Stat(otherFile); err != nil {
		t.Errorf("stat %s: %v, want the unrelated file kept", otherFile, err)
	}
}

func TestSweepIsNoOpWithoutRetention(t *testing.T) {
	client, _ := newTestClient(t, 0)
	now := time.Now()

	_, key := upload(t, client, "image/png", false)
	backdate(t, client, key, now.Add(-60*24*time.Hour))

	deleted, err := client.Sweep(now)
	if err != nil {
		t.Fatalf("Sweep() error: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}

	if _, err := client.GetObject(context.Background(), key); err != nil {
		t.Errorf("GetObject() error: %v, want the file kept", err)
	}
}

func TestSweptFileIsNoLongerServed(t *testing.T) {
	client, _ := newTestClient(t, 1)

	router := chi.NewRouter()
	client.Register(router)

	url, key := upload(t, client, "image/png", false)

	now := time.Now()
	backdate(t, client, key, now.Add(-48*time.Hour))

	if _, err := client.Sweep(now); err != nil {
		t.Fatalf("Sweep() error: %v", err)
	}

	rec := serve(t, router, http.MethodGet, url)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 after the sweep", rec.Code)
	}
}
