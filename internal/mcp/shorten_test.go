package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/shortener"
)

const shortenLongURL = "https://example.com/very/long/path?with=query"

func newShortenClient(t *testing.T, status int, body string) *shortener.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return shortener.New(server.URL, "key", 0)
}

func TestShortenReturnsShortURL(t *testing.T) {
	h := &handlers{
		log:       zapNop(),
		shortener: newShortenClient(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
	}

	result, out, err := h.shorten(context.Background(), nil, shortenInput{URL: shortenLongURL})
	if err != nil {
		t.Fatalf("shorten() error: %v", err)
	}

	if out.ShortURL != "https://short.example/abc" {
		t.Errorf("ShortURL = %q, want https://short.example/abc", out.ShortURL)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != out.ShortURL {
		t.Errorf("content = %#v, want the short URL", result.Content[0])
	}
}

func TestShortenWithoutShortener(t *testing.T) {
	h := &handlers{log: zapNop()}

	_, _, err := h.shorten(context.Background(), nil, shortenInput{URL: shortenLongURL})
	if err == nil || !strings.Contains(err.Error(), "URL shortening is not configured") {
		t.Fatalf("error = %v, want URL shortening is not configured", err)
	}
}

func TestShortenRejectsNonHTTPURL(t *testing.T) {
	h := &handlers{
		log:       zapNop(),
		shortener: newShortenClient(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
	}

	for _, input := range []string{"", "not-a-url", "example.com/x", "ftp://example.com/x", "mailto:me@example.com"} {
		if _, _, err := h.shorten(context.Background(), nil, shortenInput{URL: input}); err == nil {
			t.Errorf("shorten(%q) expected error, got nil", input)
		}
	}
}

func TestShortenUpstreamError(t *testing.T) {
	h := &handlers{
		log:       zapNop(),
		shortener: newShortenClient(t, http.StatusInternalServerError, "boom"),
	}

	_, _, err := h.shorten(context.Background(), nil, shortenInput{URL: shortenLongURL})
	if err == nil || !strings.HasPrefix(err.Error(), "shorten:") {
		t.Fatalf("error = %v, want shorten: prefix", err)
	}
}

func TestShortenRegistration(t *testing.T) {
	withShortener, err := New(Deps{
		Log:       zapNop(),
		Shortener: newShortenClient(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withShortener); !slices.Contains(names, "shorten") {
		t.Errorf("tools = %v, want shorten", names)
	}

	withoutShortener, err := New(Deps{Log: zapNop()})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withoutShortener); slices.Contains(names, "shorten") {
		t.Errorf("tools = %v, want no shorten", names)
	}
}

func TestShortenSchema(t *testing.T) {
	s := shortenSchema()

	if !slices.Contains(s.Required, "url") {
		t.Error("url is not required")
	}

	if s.Properties["url"].Default != nil {
		t.Error("url must not have a default")
	}

	if len(s.Properties) != 1 {
		t.Errorf("properties = %v, want only url", s.Properties)
	}
}

func TestShortenCallToolEndToEnd(t *testing.T) {
	var gotBody struct {
		LongLink string `json:"longlink"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"shorturl":"https://short.example/abc"}`))
	}))

	t.Cleanup(server.Close)

	srv, err := New(Deps{Log: zapNop(), Shortener: shortener.New(server.URL, "key", 0)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "shorten",
		Arguments: map[string]any{"url": shortenLongURL},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	if gotBody.LongLink != shortenLongURL {
		t.Errorf("longlink = %q, want %q", gotBody.LongLink, shortenLongURL)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "https://short.example/abc" {
		t.Errorf("content = %#v, want the short URL", result.Content[0])
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["short_url"] != "https://short.example/abc" {
		t.Errorf("structured content = %+v", result.StructuredContent)
	}
}
