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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/resolve"
)

var img2txtPNGBytes = []byte("\x89PNG\r\n\x1a\n and more")

type img2txtCapture struct {
	body []byte
}

type img2txtPayload struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Temperature *float64 `json:"temperature"`
	MaxTokens   int      `json:"max_tokens"`
	TopP        *float64 `json:"top_p"`
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

func newImg2TxtClient(t *testing.T, cfg openai.Config) *openai.Client {
	t.Helper()

	client, err := openai.New(cfg)
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

func img2txtHandlers(t *testing.T, client *openai.Client) *handlers {
	t.Helper()

	return &handlers{log: zapNop(), resolver: newImg2TxtResolver(t), openai: client}
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

func decodeImg2TxtPayload(t *testing.T, body []byte) img2txtPayload {
	t.Helper()

	var payload img2txtPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode vision payload: %v", err)
	}

	return payload
}

func img2txtUserParts(t *testing.T, body []byte) []struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL    string `json:"url"`
		Detail string `json:"detail"`
	} `json:"image_url"`
} {
	t.Helper()

	payload := decodeImg2TxtPayload(t, body)
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

func TestImg2TxtSchema(t *testing.T) {
	s := img2txtSchema()

	properties := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		properties = append(properties, name)
	}

	slices.Sort(properties)

	if !slices.Equal(properties, []string{"image", "prompt"}) {
		t.Errorf("properties = %v, want image and prompt", properties)
	}

	if !slices.Contains(s.Required, "image") {
		t.Error("image is not required")
	}

	if slices.Contains(s.Required, "prompt") {
		t.Error("prompt must be optional")
	}

	for _, field := range []string{"image", "prompt"} {
		if s.Properties[field].Default != nil {
			t.Errorf("%s must not have a default", field)
		}
	}
}

func TestImg2TxtRegisteredOnlyWhenClientPresent(t *testing.T) {
	resolver := newImg2TxtResolver(t)

	withClient, err := New(Deps{
		Log:      zapNop(),
		Resolver: resolver,
		OpenAI:   newImg2TxtClient(t, openai.Config{BaseURL: "http://example.com", Model: "m"}),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withClient); !slices.Contains(names, "img2txt") {
		t.Errorf("tools = %v, want img2txt", names)
	}

	withoutClient, err := New(Deps{Log: zapNop(), Resolver: resolver})
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

	out, err := img2txtHandlers(t, newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "default-model"})).runImg2Txt(
		context.Background(),
		img2txtInput{Image: imageServer.URL + "/x.png"})
	if err != nil {
		t.Fatalf("runImg2Txt() error: %v", err)
	}

	if out.Text != "a red square" {
		t.Errorf("Text = %q, want a red square", out.Text)
	}

	if out.Model != "default-model" {
		t.Errorf("Model = %q, want default-model", out.Model)
	}

	if out.Truncated {
		t.Error("Truncated = true, want false")
	}

	payload := decodeImg2TxtPayload(t, captured.body)

	if payload.Model != "default-model" {
		t.Errorf("payload model = %q, want default-model", payload.Model)
	}

	if payload.Temperature == nil || *payload.Temperature != 0 {
		t.Errorf("temperature = %v, want a pinned 0", payload.Temperature)
	}

	if payload.TopP != nil {
		t.Errorf("top_p = %v, want the field omitted", *payload.TopP)
	}

	if payload.MaxTokens != openai.DefaultMaxTokens {
		t.Errorf("max_tokens = %d, want %d", payload.MaxTokens, openai.DefaultMaxTokens)
	}

	if len(payload.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(payload.Messages))
	}

	parts := img2txtUserParts(t, captured.body)
	if parts[0].Text != openai.DefaultPrompt {
		t.Errorf("prompt = %q, want %q", parts[0].Text, openai.DefaultPrompt)
	}

	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img2txtPNGBytes)
	if parts[1].ImageURL.URL != wantURL {
		t.Errorf("image url = %q, want %q", parts[1].ImageURL.URL, wantURL)
	}

	if parts[1].ImageURL.Detail != "high" {
		t.Errorf("detail = %q, want high", parts[1].ImageURL.Detail)
	}
}

func TestRunImg2TxtForwardsPrompt(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, captured := newImg2TxtVisionServer(t, `{"choices":[{"message":{"content":"ok"}}]}`)

	_, err := img2txtHandlers(t, newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "m"})).runImg2Txt(
		context.Background(),
		img2txtInput{Image: imageServer.URL + "/x.png", Prompt: "read the serial number"})
	if err != nil {
		t.Fatalf("runImg2Txt() error: %v", err)
	}

	if parts := img2txtUserParts(t, captured.body); parts[0].Text != "read the serial number" {
		t.Errorf("prompt = %q, want the per-call prompt", parts[0].Text)
	}
}

func TestRunImg2TxtUsesConfiguredSystemPrompt(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, captured := newImg2TxtVisionServer(t, `{"choices":[{"message":{"content":"ok"}}]}`)

	client := newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "m", SystemPrompt: "be terse"})

	if _, err := img2txtHandlers(t, client).runImg2Txt(context.Background(), img2txtInput{
		Image: imageServer.URL + "/x.png",
	}); err != nil {
		t.Fatalf("runImg2Txt() error: %v", err)
	}

	payload := decodeImg2TxtPayload(t, captured.body)
	if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" {
		t.Fatalf("messages = %+v, want a leading system message", payload.Messages)
	}

	var systemText string
	if err := json.Unmarshal(payload.Messages[0].Content, &systemText); err != nil {
		t.Fatalf("decode system content: %v", err)
	}

	if systemText != "be terse" {
		t.Errorf("system content = %q, want be terse", systemText)
	}
}

func TestRunImg2TxtTruncated(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, _ := newImg2TxtVisionServer(t, `{"choices":[{"finish_reason":"length","message":{"content":"partial text"}}]}`)

	out, err := img2txtHandlers(t, newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "m"})).runImg2Txt(
		context.Background(),
		img2txtInput{Image: imageServer.URL + "/x.png"})
	if err != nil {
		t.Fatalf("runImg2Txt() error: %v", err)
	}

	if !out.Truncated {
		t.Error("Truncated = false, want true")
	}

	if out.Text != "partial text" {
		t.Errorf("Text = %q, want the partial text", out.Text)
	}
}

func TestRunImg2TxtResolveError(t *testing.T) {
	_, err := img2txtHandlers(t, newImg2TxtClient(t, openai.Config{BaseURL: "http://example.com", Model: "m"})).runImg2Txt(
		context.Background(),
		img2txtInput{Image: "%%%not-an-image%%%"})
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

	srv, err := New(Deps{
		Log:      zapNop(),
		Resolver: newImg2TxtResolver(t),
		OpenAI:   newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "vision-model"}),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "img2txt",
		Arguments: map[string]any{"image": imageServer.URL + "/x.png"},
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

	if structured["truncated"] != false {
		t.Errorf("structured truncated = %v, want false", structured["truncated"])
	}

	parts := img2txtUserParts(t, captured.body)
	if parts[0].Text != openai.DefaultPrompt {
		t.Errorf("prompt = %q, want %q", parts[0].Text, openai.DefaultPrompt)
	}

	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img2txtPNGBytes)
	if parts[1].ImageURL.URL != wantURL {
		t.Errorf("image url = %q, want %q", parts[1].ImageURL.URL, wantURL)
	}
}

func TestImg2TxtCallToolIgnoresRemovedFields(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, captured := newImg2TxtVisionServer(t, `{"choices":[{"message":{"content":"ok"}}]}`)

	srv, err := New(Deps{
		Log:      zapNop(),
		Resolver: newImg2TxtResolver(t),
		OpenAI:   newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "m"}),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "img2txt",
		Arguments: map[string]any{
			"image":       imageServer.URL + "/x.png",
			"temperature": 0.9,
			"top_p":       0.5,
			"detail":      "low",
			"model":       "other",
		},
	})
	if err != nil || result.IsError {
		return
	}

	payload := decodeImg2TxtPayload(t, captured.body)

	if payload.Temperature == nil || *payload.Temperature != 0 {
		t.Errorf("temperature = %v, want a pinned 0", payload.Temperature)
	}

	if payload.TopP != nil {
		t.Errorf("top_p = %v, want the field omitted", *payload.TopP)
	}

	if payload.Model != "m" {
		t.Errorf("model = %q, want the configured m", payload.Model)
	}

	if parts := img2txtUserParts(t, captured.body); parts[0].Text != openai.DefaultPrompt {
		t.Errorf("prompt = %q, want %q", parts[0].Text, openai.DefaultPrompt)
	}

	if parts := img2txtUserParts(t, captured.body); parts[1].ImageURL.Detail != "high" {
		t.Errorf("detail = %q, want high", parts[1].ImageURL.Detail)
	}
}

func TestImg2TxtCallToolReportsTruncation(t *testing.T) {
	imageServer := newImg2TxtImageServer(t)
	visionServer, _ := newImg2TxtVisionServer(t, `{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`)

	srv, err := New(Deps{
		Log:      zapNop(),
		Resolver: newImg2TxtResolver(t),
		OpenAI:   newImg2TxtClient(t, openai.Config{BaseURL: visionServer.URL, Model: "vision-model"}),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "img2txt",
		Arguments: map[string]any{"image": imageServer.URL + "/x.png"},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}

	if structured["truncated"] != true {
		t.Errorf("structured truncated = %v, want true", structured["truncated"])
	}

	if structured["text"] != "partial" {
		t.Errorf("structured text = %v, want partial", structured["text"])
	}
}
