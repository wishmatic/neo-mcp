package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func newClient(t *testing.T, cfg Config) *Client {
	t.Helper()

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return c
}

type decodedPayload struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Temperature *float64 `json:"temperature"`
	MaxTokens   int      `json:"max_tokens"`
	TopP        *float64 `json:"top_p"`
}

func decodePayload(t *testing.T, body []byte) decodedPayload {
	t.Helper()

	var payload decodedPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	return payload
}

func systemMessage(t *testing.T, body []byte) string {
	t.Helper()

	payload := decodePayload(t, body)
	if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" {
		t.Fatalf("messages = %+v, want a leading system message", payload.Messages)
	}

	var text string
	if err := json.Unmarshal(payload.Messages[0].Content, &text); err != nil {
		t.Fatalf("decode system content: %v", err)
	}

	return text
}

func userParts(t *testing.T, body []byte) []struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL    string `json:"url"`
		Detail string `json:"detail"`
	} `json:"image_url"`
} {
	t.Helper()

	payload := decodePayload(t, body)
	last := payload.Messages[len(payload.Messages)-1]
	if last.Role != "user" {
		t.Fatalf("last message role = %q, want user", last.Role)
	}

	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL    string `json:"url"`
			Detail string `json:"detail"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(last.Content, &parts); err != nil {
		t.Fatalf("decode user content: %v", err)
	}

	return parts
}

func TestDescribeSendsExpectedRequest(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"a cat"}}]}`)

	c := newClient(t, Config{
		BaseURL:      server.URL + "/",
		APIKey:       "secret",
		Model:        "default-model",
		SystemPrompt: "be terse",
		Prompt:       "what is this?",
		MaxTokens:    55,
	})

	image := []byte("png-bytes")
	result, err := c.Describe(context.Background(), DescribeRequest{ImageData: image, MediaType: "image/png"})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result.Text != "a cat" {
		t.Errorf("Text = %q, want a cat", result.Text)
	}

	if result.Model != "default-model" {
		t.Errorf("Model = %q, want default-model", result.Model)
	}

	if result.Truncated {
		t.Error("Truncated = true, want false")
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

	if payload.Temperature == nil || *payload.Temperature != 0 {
		t.Errorf("temperature = %v, want a pinned 0", payload.Temperature)
	}

	if payload.TopP != nil {
		t.Errorf("top_p = %v, want the field omitted", *payload.TopP)
	}

	if payload.MaxTokens != 55 {
		t.Errorf("max_tokens = %d, want 55", payload.MaxTokens)
	}

	if got := systemMessage(t, captured.body); got != "be terse" {
		t.Errorf("system content = %q, want be terse", got)
	}

	parts := userParts(t, captured.body)
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

func TestDescribeAppliesDefaults(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

	if _, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	payload := decodePayload(t, captured.body)

	if payload.MaxTokens != DefaultMaxTokens {
		t.Errorf("max_tokens = %d, want %d", payload.MaxTokens, DefaultMaxTokens)
	}

	if got := systemMessage(t, captured.body); got != strings.TrimSpace(DefaultSystemPrompt) {
		t.Errorf("system content = %q, want the built-in default", got)
	}

	if parts := userParts(t, captured.body); parts[0].Text != DefaultPrompt {
		t.Errorf("prompt = %q, want %q", parts[0].Text, DefaultPrompt)
	}
}

func TestDescribePromptAndSystemPromptPrecedence(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m", Prompt: "be specific", SystemPrompt: "be terse"})

	if _, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if got := systemMessage(t, captured.body); got != "be terse" {
		t.Errorf("system content = %q, want be terse", got)
	}

	if parts := userParts(t, captured.body); parts[0].Text != "be specific" {
		t.Errorf("prompt = %q, want be specific", parts[0].Text)
	}
}

func TestDescribePerCallPromptOverride(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m", Prompt: "default focus"})

	if _, err := c.Describe(context.Background(), DescribeRequest{
		ImageData: []byte("x"),
		MediaType: "image/png",
		Prompt:    "read the serial number",
	}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if parts := userParts(t, captured.body); parts[0].Text != "read the serial number" {
		t.Errorf("prompt = %q, want the per-call prompt", parts[0].Text)
	}
}

func TestDescribeMaxTokensOverride(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m", MaxTokens: 2048})

	if _, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if got := decodePayload(t, captured.body).MaxTokens; got != 2048 {
		t.Errorf("max_tokens = %d, want 2048", got)
	}

	if c.MaxTokens() != 2048 {
		t.Errorf("Client.MaxTokens() = %d, want 2048", c.MaxTokens())
	}
}

func TestDescribeOmitsAuthWhenUnset(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

	if _, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/jpeg"}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if got := captured.header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want empty", got)
	}
}

func TestDescribeNoModel(t *testing.T) {
	c := newClient(t, Config{BaseURL: "http://example.com"})

	_, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"})
	if err == nil {
		t.Fatal("Describe() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "no model configured") {
		t.Errorf("error = %q, want a no-model message", err.Error())
	}
}

func TestDescribeEmptyImage(t *testing.T) {
	c := newClient(t, Config{BaseURL: "http://example.com", Model: "m"})

	if _, err := c.Describe(context.Background(), DescribeRequest{MediaType: "image/png"}); err == nil {
		t.Fatal("Describe() expected error, got nil")
	}
}

func TestDescribeArrayContent(t *testing.T) {
	response := `{"choices":[{"message":{"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}]}`
	server, _ := newCaptureServer(t, http.StatusOK, response)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

	result, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result.Text != "hello world" {
		t.Errorf("Text = %q, want hello world", result.Text)
	}
}

func TestDescribeTruncatedWithText(t *testing.T) {
	response := `{"choices":[{"finish_reason":"length","message":{"content":"a partial cat"}}]}`
	server, _ := newCaptureServer(t, http.StatusOK, response)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

	result, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"})
	if err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if result.Text != "a partial cat" {
		t.Errorf("Text = %q, want a partial cat", result.Text)
	}

	if !result.Truncated {
		t.Error("Truncated = false, want true")
	}
}

func TestDescribeEmptyContentErrors(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		wantContains []string
	}{
		{
			name:         "length finish names truncation and the cap",
			response:     `{"choices":[{"finish_reason":"length","message":{"content":""}}]}`,
			wantContains: []string{"truncated", "2048"},
		},
		{
			name:         "reasoning content without an answer",
			response:     `{"choices":[{"finish_reason":"stop","message":{"content":"","reasoning_content":"hidden chain"}}]}`,
			wantContains: []string{"reasoning"},
		},
		{
			name:         "openrouter reasoning without an answer",
			response:     `{"choices":[{"finish_reason":"stop","message":{"content":null,"reasoning":"hidden chain"}}]}`,
			wantContains: []string{"reasoning"},
		},
		{
			name:         "content filter",
			response:     `{"choices":[{"finish_reason":"content_filter","message":{"content":null}}]}`,
			wantContains: []string{"content filter"},
		},
		{
			name:         "null content includes finish reason and snippet",
			response:     `{"choices":[{"finish_reason":"stop","message":{"content":null}}]}`,
			wantContains: []string{`finish_reason "stop"`, `"content":null`},
		},
		{
			name:         "non-text parts include finish reason and snippet",
			response:     `{"choices":[{"finish_reason":"stop","message":{"content":[{"type":"image_url","image_url":{"url":"x"}}]}}]}`,
			wantContains: []string{`finish_reason "stop"`, `"image_url"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newCaptureServer(t, http.StatusOK, tt.response)

			c := newClient(t, Config{BaseURL: server.URL, Model: "m", MaxTokens: 2048})

			_, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"})
			if err == nil {
				t.Fatal("Describe() expected error, got nil")
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want substring %q", err.Error(), want)
				}
			}

			if strings.Contains(err.Error(), "hidden chain") {
				t.Errorf("error = %q, must not leak reasoning content", err.Error())
			}
		})
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

			c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

			_, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"})
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

	c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Describe(ctx, DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"}); err == nil {
		t.Fatal("Describe() expected error, got nil")
	}
}

func TestMaxTokensLabel(t *testing.T) {
	if got := maxTokensLabel(0); got != "unset" {
		t.Errorf("maxTokensLabel(0) = %q, want unset", got)
	}

	if got := maxTokensLabel(DefaultMaxTokens); got != strconv.Itoa(DefaultMaxTokens) {
		t.Errorf("maxTokensLabel(%d) = %q", DefaultMaxTokens, got)
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
			if _, err := New(Config{BaseURL: tt.baseURL, Model: "m"}); err == nil {
				t.Fatal("New() expected error, got nil")
			}
		})
	}
}
