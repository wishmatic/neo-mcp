package filestore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter(t *testing.T) (*Client, chi.Router) {
	t.Helper()

	client, _ := newTestClient(t)

	router := chi.NewRouter()
	client.Register(router)

	return client, router
}

func serve(t *testing.T, router http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, target, nil))

	return rec
}

func storedURL(t *testing.T, client *Client) string {
	t.Helper()

	url, err := client.UploadFile(context.Background(), []byte("image-bytes"), "image/png")
	if err != nil {
		t.Fatalf("UploadFile() error: %v", err)
	}

	return url
}

func TestServeFile(t *testing.T) {
	client, router := newTestRouter(t)

	rec := serve(t, router, http.MethodGet, storedURL(t, client))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if rec.Body.String() != "image-bytes" {
		t.Errorf("body = %q, want image-bytes", rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}

	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want an immutable public cache", cc)
	}

	if xcto := rec.Header().Get("X-Content-Type-Options"); xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", xcto)
	}
}

func TestServeHead(t *testing.T) {
	client, router := newTestRouter(t)

	rec := serve(t, router, http.MethodHead, storedURL(t, client))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}

	if length := rec.Header().Get("Content-Length"); length != strconv.Itoa(len("image-bytes")) {
		t.Errorf("Content-Length = %q, want %d", length, len("image-bytes"))
	}

	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}

func TestServeRange(t *testing.T) {
	client, router := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, storedURL(t, client), nil)
	req.Header.Set("Range", "bytes=0-4")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}

	if rec.Body.String() != "image" {
		t.Errorf("body = %q, want image", rec.Body.String())
	}
}

func TestServeNotFound(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "missing key", target: "https://neo.example.com/i/2026-09/missing.png"},
		{name: "traversal", target: "https://neo.example.com/i/../../etc/passwd.png"},
		{name: "encoded traversal", target: "https://neo.example.com/i/%2e%2e/%2e%2e/etc/passwd.png"},
		{name: "unknown extension", target: "https://neo.example.com/i/2026-09/x.gif"},
		{name: "other namespace", target: "https://neo.example.com/a/avatars/x.png"},
		{name: "no extension", target: "https://neo.example.com/i/2026-09/x"},
	}

	client, router := newTestRouter(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, router, http.MethodGet, tt.target)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}

			if strings.Contains(rec.Body.String(), client.cfg.Dir) {
				t.Errorf("body %q leaks the storage directory", rec.Body.String())
			}
		})
	}
}

func TestServeDeletedFile(t *testing.T) {
	client, router := newTestRouter(t)
	url := storedURL(t, client)

	if rec := serve(t, router, http.MethodGet, url); rec.Code != http.StatusOK {
		t.Fatalf("status before deletion = %d, want 200", rec.Code)
	}

	key := strings.TrimPrefix(url, "https://neo.example.com/")

	path, err := client.safePath(key)
	if err != nil {
		t.Fatalf("safePath() error: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	if rec := serve(t, router, http.MethodGet, url); rec.Code != http.StatusNotFound {
		t.Fatalf("status after deletion = %d, want 404", rec.Code)
	}
}
