package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capturedRequest struct {
	method string
	path   string
	header http.Header
	body   []byte
}

func newCaptureServer(t *testing.T, status int, response string) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.header = r.Header.Clone()
		captured.body, _ = io.ReadAll(r.Body)

		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(server.Close)

	return server, captured
}

type decodedPayload struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`
}

func decodePayload(t *testing.T, body []byte) decodedPayload {
	t.Helper()

	var payload decodedPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	return payload
}

func TestDescribeSendsExpectedRequest(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"a cat"}}]}`)

	c, err := New(server.URL+"/", "secret", "default-model")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	image := []byte("png-bytes")
	result, err := c.Describe(context.Background(), DescribeRequest{
		ImageData:    image,
		MediaType:    "image/png",
		Prompt:       "what is this?",
		SystemPrompt: "be terse",
		Temperature:  0.3,
		MaxTokens:    55,
		TopP:         0.9,
		Detail:       "high",
	})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result.Text != "a cat" {
		t.Errorf("Text = %q, want a cat", result.Text)
	}

	if result.Model != "default-model" {
		t.Errorf("Model = %q, want default-model", result.Model)
	}

	if captured.method != http.MethodPost {
		t.Errorf("method = %q, want POST", captured.method)
	}

	if captured.path != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", captured.path)
	}

	if got := captured.header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	if got := captured.header.Get("Authorization"); got != "Bearer secret" {
		t.Errorf("Authorization = %q, want Bearer secret", got)
	}

	payload := decodePayload(t, captured.body)

	if payload.Model != "default-model" {
		t.Errorf("payload model = %q, want default-model", payload.Model)
	}

	if payload.Temperature != 0.3 || payload.MaxTokens != 55 || payload.TopP != 0.9 {
		t.Errorf("tuning = (%v, %v, %v), want (0.3, 55, 0.9)", payload.Temperature, payload.MaxTokens, payload.TopP)
	}

	if len(payload.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(payload.Messages))
	}

	if payload.Messages[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want system", payload.Messages[0].Role)
	}

	var systemText string
	if err := json.Unmarshal(payload.Messages[0].Content, &systemText); err != nil {
		t.Fatalf("decode system content: %v", err)
	}

	if systemText != "be terse" {
		t.Errorf("system content = %q, want be terse", systemText)
	}

	if payload.Messages[1].Role != "user" {
		t.Errorf("messages[1].role = %q, want user", payload.Messages[1].Role)
	}

	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL    string `json:"url"`
			Detail string `json:"detail"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(payload.Messages[1].Content, &parts); err != nil {
		t.Fatalf("decode user content: %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("user parts = %d, want 2", len(parts))
	}

	if parts[0].Type != "text" || parts[0].Text != "what is this?" {
		t.Errorf("text part = %+v, want text/what is this?", parts[0])
	}

	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
	if parts[1].Type != "image_url" || parts[1].ImageURL.URL != wantURL || parts[1].ImageURL.Detail != "high" {
		t.Errorf("image part = %+v, want image_url/%s/high", parts[1], wantURL)
	}
}

func TestDescribeOmitsAuthAndSystemWhenUnset(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c, err := New(server.URL, "", "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := c.Describe(context.Background(), DescribeRequest{
		ImageData: []byte("x"),
		MediaType: "image/jpeg",
		Prompt:    "hi",
		Model:     "m",
	}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if got := captured.header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want empty", got)
	}

	payload := decodePayload(t, captured.body)
	if len(payload.Messages) != 1 || payload.Messages[0].Role != "user" {
		t.Errorf("messages = %+v, want a single user message", payload.Messages)
	}
}

func TestDescribeModelOverride(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c, err := New(server.URL, "", "default")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := c.Describe(context.Background(), DescribeRequest{
		ImageData: []byte("x"),
		MediaType: "image/png",
		Prompt:    "hi",
		Model:     "override",
	})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result.Model != "override" {
		t.Errorf("Model = %q, want override", result.Model)
	}

	if got := decodePayload(t, captured.body).Model; got != "override" {
		t.Errorf("payload model = %q, want override", got)
	}
}

func TestDescribeNoModel(t *testing.T) {
	c, err := New("http://example.com", "", "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := c.Describe(context.Background(), DescribeRequest{
		ImageData: []byte("x"),
		MediaType: "image/png",
		Prompt:    "hi",
	}); err == nil {
		t.Fatal("Describe() expected error, got nil")
	}
}

func TestDescribeEmptyImage(t *testing.T) {
	c, err := New("http://example.com", "", "m")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := c.Describe(context.Background(), DescribeRequest{MediaType: "image/png", Prompt: "hi"}); err == nil {
		t.Fatal("Describe() expected error, got nil")
	}
}

func TestDescribeArrayContent(t *testing.T) {
	response := `{"choices":[{"message":{"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}]}`
	server, _ := newCaptureServer(t, http.StatusOK, response)

	c, err := New(server.URL, "", "m")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := c.Describe(context.Background(), DescribeRequest{
		ImageData: []byte("x"),
		MediaType: "image/png",
		Prompt:    "hi",
	})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result.Text != "hello world" {
		t.Errorf("Text = %q, want hello world", result.Text)
	}
}

func TestDescribeErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		response   string
		wantSubstr string
	}{
		{"upstream error message", http.StatusUnauthorized, `{"error":{"message":"bad key"}}`, "bad key"},
		{"plain body", http.StatusInternalServerError, "oops", "oops"},
		{"no choices", http.StatusOK, `{"choices":[]}`, "no choices"},
		{"malformed json", http.StatusOK, "not json", "decode response"},
		{"empty text", http.StatusOK, `{"choices":[{"message":{"content":""}}]}`, "no text"},
		{"bad content type", http.StatusOK, `{"choices":[{"message":{"content":123}}]}`, "decode message content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newCaptureServer(t, tt.status, tt.response)

			c, err := New(server.URL, "", "m")
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			_, err = c.Describe(context.Background(), DescribeRequest{
				ImageData: []byte("x"),
				MediaType: "image/png",
				Prompt:    "hi",
			})
			if err == nil {
				t.Fatal("Describe() expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSubstr)
			}
		})
	}
}

func TestDescribeContextCancelled(t *testing.T) {
	server, _ := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"x"}}]}`)

	c, err := New(server.URL, "", "m")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Describe(ctx, DescribeRequest{ImageData: []byte("x"), MediaType: "image/png", Prompt: "hi"}); err == nil {
		t.Fatal("Describe() expected error, got nil")
	}
}

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"empty base url", ""},
		{"missing scheme", "example.com"},
		{"unsupported scheme", "ftp://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.baseURL, "", "m"); err == nil {
				t.Fatal("New() expected error, got nil")
			}
		})
	}
}
