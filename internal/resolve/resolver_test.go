package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeStore struct {
	key  string
	data []byte
}

func (f *fakeStore) GetObject(_ context.Context, key string) ([]byte, error) {
	f.key = key

	return f.data, nil
}

func TestFetchStoredURLReadsFromStore(t *testing.T) {
	store := &fakeStore{data: []byte("from-store")}

	r, err := New(store, "https://cdn.example.com", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	data, err := r.Fetch(context.Background(), "https://cdn.example.com/i/2026-09/x.png")
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if string(data) != "from-store" {
		t.Errorf("data = %q, want from-store", data)
	}

	if store.key != "i/2026-09/x.png" {
		t.Errorf("key = %q, want i/2026-09/x.png", store.key)
	}
}

func TestFetchFollowsRedirectToStoredURL(t *testing.T) {
	store := &fakeStore{data: []byte("from-store")}

	r, err := New(store, "https://cdn.example.com", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://cdn.example.com/i/2026-09/x.png", http.StatusFound)
	}))
	defer redirector.Close()

	data, err := r.Fetch(context.Background(), redirector.URL+"/abc")
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if string(data) != "from-store" {
		t.Errorf("data = %q, want from-store", data)
	}

	if store.key != "i/2026-09/x.png" {
		t.Errorf("key = %q, want i/2026-09/x.png", store.key)
	}
}

func TestFetchFollowsRedirectToPlainURL(t *testing.T) {
	r, err := New(nil, "", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("from-http"))
	}))
	defer origin.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, origin.URL+"/x.png", http.StatusMovedPermanently)
	}))
	defer redirector.Close()

	data, err := r.Fetch(context.Background(), redirector.URL)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if string(data) != "from-http" {
		t.Errorf("data = %q, want from-http", data)
	}
}

func TestFetchSetsUserAgent(t *testing.T) {
	var (
		mu     sync.Mutex
		agents []string
	)

	record := func(req *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		agents = append(agents, req.UserAgent())
	}

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		record(req)
		_, _ = w.Write([]byte("from-http"))
	}))
	defer origin.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		record(req)
		http.Redirect(w, req, origin.URL+"/x.png", http.StatusFound)
	}))
	defer redirector.Close()

	r, err := New(nil, "", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := r.Fetch(context.Background(), redirector.URL+"/start"); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(agents) != 2 {
		t.Fatalf("requests = %d, want one per redirect hop", len(agents))
	}

	for i, agent := range agents {
		if !strings.Contains(agent, "neo-mcp") || !strings.Contains(agent, "bot") {
			t.Errorf("hop %d User-Agent = %q, want a descriptive agent naming neo-mcp and bot", i, agent)
		}
	}
}

func TestFetchStoredURLWithoutStore(t *testing.T) {
	r, err := New(nil, "https://cdn.example.com", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := r.Fetch(context.Background(), "https://cdn.example.com/i/x.png"); err == nil {
		t.Fatal("Fetch() expected error, got nil")
	}
}

func TestFetchIgnoresOtherNamespaces(t *testing.T) {
	store := &fakeStore{data: []byte("from-store")}

	r, err := New(store, "https://cdn.example.com", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := r.Fetch(context.Background(), "https://cdn.example.com/a/avatars/x.png"); err == nil {
		t.Fatal("Fetch() expected error, got nil")
	}

	if store.key != "" {
		t.Errorf("key = %q, want no store access", store.key)
	}
}

func TestFetchNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	r, err := New(nil, "", nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := r.Fetch(context.Background(), server.URL+"/missing.png"); err == nil {
		t.Fatal("Fetch() expected error, got nil")
	}
}

func TestNewInvalidPublicBase(t *testing.T) {
	if _, err := New(nil, "cdn.example.com", nil); err == nil {
		t.Fatal("New() expected error, got nil")
	}
}
