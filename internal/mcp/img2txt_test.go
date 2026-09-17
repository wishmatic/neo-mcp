package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/resolve"
)

var img2txtPNGBytes = []byte("\x89PNG\r\n\x1a\n and more")

type img2txtCapture struct {
	body []byte
}

func newImg2TxtImageServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(img2txtPNGBytes)
	}))

	t.Cleanup(server.Close)

	return server
}

func newImg2TxtVisionServer(t *testing.T, response string) (*httptest.Server, *img2txtCapture) {
	t.Helper()

	captured := &img2txtCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.body, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(server.Close)

	return server, captured
}

func newImg2TxtClient(t *testing.T, baseURL, model string) *openai.Client {
	t.Helper()

	client, err := openai.New(baseURL, "", model, 5*time.Second)
	if err != nil {
		t.Fatalf("openai.New() error: %v", err)
	}

	return client
}

func newImg2TxtResolver(t *testing.T) *resolve.Resolver {
	t.Helper()

	resolver, err := resolve.New(nil, "")
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	return resolver
}

func connectSession(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect() error: %v", err)
	}

	t.Cleanup(func() { _ = serverSession.Close() })

	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect() error: %v", err)
	}

	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}

func toolNames(t *testing.T, srv *mcp.Server) []string {
	t.Helper()

	result, err := connectSession(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}

	return names
}

func TestImg2TxtSchema(t *testing.T) {
	s := img2txtSchema()

	if !slices.Contains(s.Required, "image") {
		t.Error("image is not required")
	}

	if s.Properties["image"].Default != nil {
		t.Error("image must not have a default")
	}

	defaults := map[string]string{
		"prompt":      `"Describe this image in detail."`,
		"temperature": "0.2",
		"max_tokens":  "1024",
		"top_p":       "1",
		"detail":      `"auto"`,
	}

	for field, want := range defaults {
		prop := s.Properties[field]
		if prop == nil {
			t.Errorf("%s property is missing", field)
			continue
		}

		if prop.Default == nil {
			t.Errorf("%s has no default", field)
			continue
		}

		if string(prop.Default) != want {
			t.Errorf("%s default = %s, want %s", field, prop.Default, want)
		}
	}

	for _, field := range []string{"model", "system_prompt"} {
		if s.Properties[field].Default != nil {
			t.Errorf("%s must not have a default", field)
		}
	}

	detailEnum := s.Properties["detail"].Enum
	if len(detailEnum) != 3 {
		t.Fatalf("detail enum = %v, want three values", detailEnum)
	}

	for i, want := range []string{"auto", "low", "high"} {
		if detailEnum[i] != want {
			t.Errorf("detail enum[%d] = %v, want %q", i, detailEnum[i], want)
		}
	}
}

func TestImg2TxtRegisteredOnlyWhenClientPresent(t *testing.T) {
	resolver := newImg2TxtResolver(t)

	withClient, err := New(zapNop(), nil, nil, nil, resolver, newImg2TxtClient(t, "http://example.com", "m"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withClient); !slices.Contains(names, "img2txt") {
		t.Errorf("tools = %v, want img2txt", names)
	}

	withoutClient, err := New(zapNop(), nil, nil, nil, resolver, nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withoutClient); slices.Contains(names, "img2txt") {
		t.Errorf("tools = %v, want no img2txt", names)
	}
}

func TestRunImg2TxtResolvesAndForwards(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, captured := newImg2TxtVisionServer(t, `{"choices":[{"message":{"content":"a red square"}}]}`)

	out, err := runImg2Txt(context.Background(), zapNop(), newImg2TxtResolver(t),
		newImg2TxtClient(t, visionServer.URL, "default-model"),
		img2txtInput{
			Image:        imageServer.URL + "/x.png",
			Prompt:       "what is it?",
			SystemPrompt: "be terse",
			Temperature:  0.3,
			MaxTokens:    55,
			TopP:         0.9,
			Detail:       "high",
		})
	if err != nil {
		t.Fatalf("runImg2Txt() error: %v", err)
	}

	if out.Text != "a red square" {
		t.Errorf("Text = %q, want a red square", out.Text)
	}

	if out.Model != "default-model" {
		t.Errorf("Model = %q, want default-model", out.Model)
	}

	var payload struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Temperature float64 `json:"temperature"`
		MaxTokens   int     `json:"max_tokens"`
		TopP        float64 `json:"top_p"`
	}
	if err := json.Unmarshal(captured.body, &payload); err != nil {
		t.Fatalf("decode vision payload: %v", err)
	}

	if payload.Temperature != 0.3 || payload.MaxTokens != 55 || payload.TopP != 0.9 {
		t.Errorf("tuning = (%v, %v, %v), want (0.3, 55, 0.9)", payload.Temperature, payload.MaxTokens, payload.TopP)
	}

	if len(payload.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(payload.Messages))
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

	if parts[0].Text != "what is it?" {
		t.Errorf("prompt = %q, want what is it?", parts[0].Text)
	}

	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img2txtPNGBytes)
	if parts[1].ImageURL.URL != wantURL {
		t.Errorf("image url = %q, want %q", parts[1].ImageURL.URL, wantURL)
	}

	if parts[1].ImageURL.Detail != "high" {
		t.Errorf("detail = %q, want high", parts[1].ImageURL.Detail)
	}
}

func TestRunImg2TxtResolveError(t *testing.T) {
	_, err := runImg2Txt(context.Background(), zapNop(), newImg2TxtResolver(t),
		newImg2TxtClient(t, "http://example.com", "m"),
		img2txtInput{Image: "%%%not-an-image%%%", Prompt: "hi"})
	if err == nil {
		t.Fatal("runImg2Txt() expected error, got nil")
	}

	if !strings.HasPrefix(err.Error(), "img2txt: resolve image:") {
		t.Errorf("error = %q, want img2txt: resolve image: prefix", err.Error())
	}
}

func TestImg2TxtCallToolEndToEnd(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, captured := newImg2TxtVisionServer(t, `{"choices":[{"message":{"content":"a red square"}}]}`)

	srv, err := New(zapNop(), nil, nil, nil, newImg2TxtResolver(t), newImg2TxtClient(t, visionServer.URL, "vision-model"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "img2txt",
		Arguments: map[string]any{
			"image":  imageServer.URL + "/x.png",
			"prompt": "what is it?",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", result.Content[0])
	}

	if text.Text != "a red square" {
		t.Errorf("text = %q, want a red square", text.Text)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}

	if structured["text"] != "a red square" || structured["model"] != "vision-model" {
		t.Errorf("structured content = %+v", structured)
	}

	var payload struct {
		Messages []struct {
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(captured.body, &payload); err != nil {
		t.Fatalf("decode vision payload: %v", err)
	}

	parts := payload.Messages[0].Content
	if len(parts) != 2 {
		t.Fatalf("content parts = %d, want 2", len(parts))
	}

	if parts[0].Text != "what is it?" {
		t.Errorf("prompt = %q, want what is it?", parts[0].Text)
	}

	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img2txtPNGBytes)
	if parts[1].ImageURL.URL != wantURL {
		t.Errorf("image url = %q, want %q", parts[1].ImageURL.URL, wantURL)
	}
}
