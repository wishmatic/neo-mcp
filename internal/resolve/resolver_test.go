package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestFetchGaragefrontURLReadsFromS3(t *testing.T) {
	store := &fakeStore{data: []byte("from-s3")}

	r, err := New(store, "https://cdn.example.com")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	data, err := r.Fetch(context.Background(), "https://cdn.example.com/i/images/abc/2026-09/x.png")
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if string(data) != "from-s3" {
		t.Errorf("data = %q, want from-s3", data)
	}

	if store.key != "i/images/abc/2026-09/x.png" {
		t.Errorf("key = %q, want i/images/abc/2026-09/x.png", store.key)
	}
}

func TestFetchFollowsRedirectToGaragefront(t *testing.T) {
	store := &fakeStore{data: []byte("from-s3")}

	r, err := New(store, "https://cdn.example.com")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	shortener := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://cdn.example.com/a/avatars/abc/x.png", http.StatusFound)
	}))
	defer shortener.Close()

	data, err := r.Fetch(context.Background(), shortener.URL+"/abc")
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if string(data) != "from-s3" {
		t.Errorf("data = %q, want from-s3", data)
	}

	if store.key != "a/avatars/abc/x.png" {
		t.Errorf("key = %q, want a/avatars/abc/x.png", store.key)
	}
}

func TestFetchFollowsRedirectToPlainURL(t *testing.T) {
	r, err := New(nil, "")
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

func TestFetchGaragefrontURLWithoutS3(t *testing.T) {
	r, err := New(nil, "https://cdn.example.com")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := r.Fetch(context.Background(), "https://cdn.example.com/i/x.png"); err == nil {
		t.Fatal("Fetch() expected error, got nil")
	}
}

func TestFetchNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	r, err := New(nil, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := r.Fetch(context.Background(), server.URL+"/missing.png"); err == nil {
		t.Fatal("Fetch() expected error, got nil")
	}
}

func TestNewInvalidPublicBase(t *testing.T) {
	if _, err := New(nil, "cdn.example.com"); err == nil {
		t.Fatal("New() expected error, got nil")
	}
}
