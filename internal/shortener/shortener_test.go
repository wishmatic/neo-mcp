package shortener

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type newRequestBody struct {
	LongLink    string `json:"longlink"`
	ExpiryDelay int    `json:"expiry_delay"`
}

func TestShorten(t *testing.T) {
	var gotKey, gotPath string
	var gotBody newRequestBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"error":false,"shorturl":"https://s.example/abc123","expiry_time":0}`))
	}))

	defer srv.Close()

	c := New(srv.URL, "secret-key", 0)
	got, err := c.Shorten(context.Background(), "https://example.com/very/long/path")
	if err != nil {
		t.Fatalf("Shorten() error: %v", err)
	}

	if got != "https://s.example/abc123" {
		t.Errorf("Shorten() = %q, want %q", got, "https://s.example/abc123")
	}

	if gotKey != "secret-key" {
		t.Errorf("X-API-Key = %q, want %q", gotKey, "secret-key")
	}

	if gotPath != "/api/new" {
		t.Errorf("path = %q, want %q", gotPath, "/api/new")
	}

	if gotBody.LongLink != "https://example.com/very/long/path" {
		t.Errorf("longlink = %q", gotBody.LongLink)
	}

	if gotBody.ExpiryDelay != 0 {
		t.Errorf("expiry_delay = %d, want 0", gotBody.ExpiryDelay)
	}
}

func TestShortenIncludesExpiryDelay(t *testing.T) {
	var gotBody newRequestBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"error":false,"shorturl":"abc","expiry_time":0}`))
	}))

	defer srv.Close()

	c := New(srv.URL, "secret-key", 3600)
	if _, err := c.Shorten(context.Background(), "https://example.com/long"); err != nil {
		t.Fatalf("Shorten() error: %v", err)
	}

	if gotBody.ExpiryDelay != 3600 {
		t.Errorf("expiry_delay = %d, want 3600", gotBody.ExpiryDelay)
	}
}

func TestShortenResolvesRelativeURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"error":false,"shorturl":"/abc123","expiry_time":0}`))
	}))

	defer srv.Close()

	c := New(srv.URL, "secret-key", 0)
	got, err := c.Shorten(context.Background(), "https://example.com/long")
	if err != nil {
		t.Fatalf("Shorten() error: %v", err)
	}

	if got != srv.URL+"/abc123" {
		t.Errorf("Shorten() = %q, want %q", got, srv.URL+"/abc123")
	}
}

func TestShortenAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error":true,"reason":"something broke"}`))
	}))

	defer srv.Close()

	c := New(srv.URL, "secret-key", 0)
	if _, err := c.Shorten(context.Background(), "https://example.com/long"); err == nil {
		t.Fatal("Shorten() expected error, got nil")
	}
}

func TestShortenNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))

	defer srv.Close()

	c := New(srv.URL, "wrong-key", 0)
	if _, err := c.Shorten(context.Background(), "https://example.com/long"); err == nil {
		t.Fatal("Shorten() expected error, got nil")
	}
}
