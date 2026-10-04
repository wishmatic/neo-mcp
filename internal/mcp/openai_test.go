package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/openai"
	"github.com/wishmatic/neo-mcp/internal/present"
)

func newOpenAIServer(t *testing.T, log *requestLog) *mcp.Server {
	t.Helper()

	return newOpenAIServerWith(t, newOpenAIBackend(t, log))
}

func newOpenAIServerWith(t *testing.T, client *openai.Client) *mcp.Server {
	t.Helper()

	srv, err := New(Clients{
		Log:      zapNop(),
		OpenAI:   client,
		Store:    newTestStore(t),
		Resolver: newResolver(t),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return srv
}

func structuredOutput(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	return decodeJSONBody(t, raw)
}

func TestOpenAICallToolStoresAndAttaches(t *testing.T) {
	log := &requestLog{}
	srv := newOpenAIServer(t, log)

	result := callTool(t, srv, "openai", map[string]any{
		"model":  "flux-schnell",
		"prompt": "a cat",
		"size":   "1024x1024",
	})

	if path := singleRequestPath(t, log); path != "/images/generations" {
		t.Errorf("path = %q, want /images/generations", path)
	}

	body := singleRequestBody(t, log)

	if body["model"] != "flux-schnell" || body["prompt"] != "a cat" || body["size"] != "1024x1024" {
		t.Errorf("body = %v, want the model, prompt, and size copied", body)
	}

	if body["response_format"] != openai.ResponseFormatBase64 {
		t.Errorf("response_format = %v, want %s", body["response_format"], openai.ResponseFormatBase64)
	}

	if len(result.Content) != 2 {
		t.Fatalf("content = %d, want a URL and one image", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.HasSuffix(text.Text, ".webp") {
		t.Fatalf("content[0] = %#v, want an uploaded .webp URL", result.Content[0])
	}

	img, ok := result.Content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
	}

	if img.MIMEType != "image/webp" {
		t.Errorf("mime type = %q, want image/webp", img.MIMEType)
	}

	if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, []mcp.Role{present.RoleUser, present.RoleAssistant}) {
		t.Errorf("audience = %+v, want [user assistant]", img.Annotations)
	}

	out := structuredOutput(t, result)

	if out["provider"] != "primary" || out["model"] != "flux-schnell" {
		t.Errorf("provider = %v, model = %v, want primary and flux-schnell", out["provider"], out["model"])
	}

	if out["count"] != float64(1) || out["costUsd"] != 0.003 {
		t.Errorf("count = %v, costUsd = %v, want 1 at 0.003", out["count"], out["costUsd"])
	}

	urls, ok := out["urls"].([]any)
	if !ok || len(urls) != 1 || urls[0] != text.Text {
		t.Errorf("urls = %v, want the one URL the result carries as text", out["urls"])
	}
}

func TestOpenAICallToolPassesTheOptionalFields(t *testing.T) {
	log := &requestLog{}
	srv := newOpenAIServer(t, log)

	callTool(t, srv, "openai", map[string]any{
		"prompt":              "a cat",
		"n":                   2,
		"seed":                42,
		"strength":            0.6,
		"guidance_scale":      3.5,
		"num_inference_steps": 28,
	})

	body := singleRequestBody(t, log)

	for field, want := range map[string]float64{
		"n": 2, "seed": 42, "strength": 0.6, "guidance_scale": 3.5, "num_inference_steps": 28,
	} {
		if got := numberField(t, body, field); got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}
}

func TestOpenAICallToolSelectsTheProvider(t *testing.T) {
	firstLog, secondLog := &requestLog{}, &requestLog{}

	first := newOpenAIProvider(t, firstLog, openaiInlineResponse(t, testImagePNG(t)))

	second := newOpenAIProvider(t, secondLog, openaiInlineResponse(t, testImagePNG(t)))
	second.Name = "other"

	srv := newOpenAIServerWith(t, openai.New([]openai.Provider{first, second}))

	callTool(t, srv, "openai", map[string]any{"prompt": "a cat"})

	if paths, _ := firstLog.snapshot(); len(paths) != 1 {
		t.Errorf("the first provider saw %d requests, want the default call", len(paths))
	}

	if paths, _ := secondLog.snapshot(); len(paths) != 0 {
		t.Errorf("the second provider saw %d requests, want none", len(paths))
	}

	result := callTool(t, srv, "openai", map[string]any{"prompt": "a cat", "provider": "other"})

	if paths, _ := secondLog.snapshot(); len(paths) != 1 {
		t.Errorf("the second provider saw %d requests, want the named call", len(paths))
	}

	if out := structuredOutput(t, result); out["provider"] != "other" {
		t.Errorf("provider = %v, want the named other", out["provider"])
	}
}

// A name that is not configured is refused by the schema's enum before the handler runs, so the call never reaches a
// provider.
func TestOpenAIUnknownProviderFailsWithoutARequest(t *testing.T) {
	log := &requestLog{}
	srv := newOpenAIServer(t, log)

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "openai",
		Arguments: map[string]any{"prompt": "a cat", "provider": "nope"},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("result = %+v, want a tool error for an unknown provider", result.Content)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "nope") || !strings.Contains(text.Text, "primary") {
		t.Fatalf("content[0] = %#v, want an error naming the provider and the configured ones", result.Content[0])
	}

	if paths, _ := log.snapshot(); len(paths) != 0 {
		t.Errorf("requests = %d, want none for an unknown provider", len(paths))
	}
}

func TestOpenAICallToolReadsTheInputImages(t *testing.T) {
	log := &requestLog{}
	srv := newOpenAIServer(t, log)

	image := newSizedInitImageURL(t, testImageSize, testImageSize)

	callTool(t, srv, "openai", map[string]any{
		"prompt": "a cat",
		"image":  image,
		"images": []string{image},
		"mask":   image,
	})

	body := singleRequestBody(t, log)

	for _, field := range []string{"imageDataUrl", "maskDataUrl"} {
		inline, ok := body[field].(string)
		if !ok {
			t.Fatalf("%s = %T, want an inline data URL", field, body[field])
		}

		decodeInlineImage(t, field, inline)
	}

	images, ok := body["imageDataUrls"].([]any)
	if !ok || len(images) != 1 {
		t.Fatalf("imageDataUrls = %v, want the one input image", body["imageDataUrls"])
	}

	decodeInlineImage(t, "imageDataUrls[0]", images[0].(string))
}

func TestOpenAICallToolFetchesALinkedImage(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testImagePNG(t))
	}))
	t.Cleanup(images.Close)

	log := &requestLog{}
	srv := newOpenAIServerWith(t, openai.New([]openai.Provider{
		newOpenAIProvider(t, log, `{"data":[{"url":"`+images.URL+`/a.png"}]}`),
	}))

	result := callTool(t, srv, "openai", map[string]any{"prompt": "a cat"})

	if len(result.Content) != 2 {
		t.Fatalf("content = %d, want a URL and one image", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.HasSuffix(text.Text, ".webp") {
		t.Fatalf("content[0] = %#v, want the linked image stored as the output format", result.Content[0])
	}
}

func TestOpenAICallToolErrors(t *testing.T) {
	tests := []struct {
		name     string
		response string
		status   int
	}{
		{name: "no images", response: `{"data":[]}`, status: http.StatusOK},
		{name: "http error", response: `{"error":{"message":"bad key"}}`, status: http.StatusUnauthorized},
	}

	h := &Clients{Log: zapNop(), Store: newTestStore(t), Resolver: newResolver(t)}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			}))
			t.Cleanup(server.Close)

			h.OpenAI = openai.New([]openai.Provider{{Name: "primary", BaseURL: server.URL}})

			_, _, err := h.openai(context.Background(), nil, openaiInput{Prompt: "a cat"})
			if err == nil || !strings.HasPrefix(err.Error(), "openai:") {
				t.Fatalf("error = %v, want a openai: prefix", err)
			}
		})
	}
}

func TestOpenAISchema(t *testing.T) {
	s := openaiSchema(format.WebP, []string{"primary", "other"})

	if !slices.Contains(s.Required, "prompt") {
		t.Error("openai: prompt is not required")
	}

	if slices.Contains(s.Required, "provider") {
		t.Error("openai: provider is required, want it to default to the first configured")
	}

	provider := s.Properties["provider"]
	if provider == nil {
		t.Fatal("openai: provider property is missing")
	}

	if !slices.Equal(provider.Enum, []any{"primary", "other"}) {
		t.Errorf("provider enum = %v, want the configured names in order", provider.Enum)
	}

	if string(provider.Default) != `"primary"` {
		t.Errorf("provider default = %s, want the first configured", provider.Default)
	}

	if got := string(s.Properties["format"].Default); got != `"webp"` {
		t.Errorf("format default = %s, want the server's configured webp", got)
	}

	if got := string(s.Properties["inline_max_edge"].Default); got != fmt.Sprint(present.DefaultInlineMaxEdge) {
		t.Errorf("inline_max_edge default = %s, want %d", got, present.DefaultInlineMaxEdge)
	}
}

func TestOpenAIWithoutProvidersHasNoEnum(t *testing.T) {
	s := openaiSchema(format.WebP, nil)

	if provider := s.Properties["provider"]; provider.Enum != nil || provider.Default != nil {
		t.Errorf("provider = %+v, want no enum or default without configured providers", provider)
	}
}

func TestOpenAIOutputSchemaCarriesTheSharedFields(t *testing.T) {
	s, err := jsonschema.For[openaiOutput](nil)
	if err != nil {
		t.Fatalf("jsonschema.For() error: %v", err)
	}

	for _, field := range []string{"provider", "model", "count", "urls", "costUsd"} {
		if s.Properties[field] == nil {
			t.Errorf("openai: output %s is missing", field)
		}
	}
}

func TestOpenAILogsDoNotLeakSecrets(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\n")
	inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)

	log, logs := observedLogger()

	h := &Clients{
		Log:      log,
		OpenAI:   newOpenAIBackend(t, &requestLog{}),
		Store:    newTestStore(t),
		Resolver: newResolver(t),
	}

	in := openaiInput{Prompt: "a cat", Model: "flux-schnell", Image: inline}

	if _, _, err := h.openai(context.Background(), nil, in); err != nil {
		t.Fatalf("openai() error: %v", err)
	}

	failing := &Clients{
		Log:    log,
		OpenAI: openai.New([]openai.Provider{{Name: "primary", BaseURL: "http://127.0.0.1:1", APIKey: "sk-test"}}),
		Store:  newTestStore(t),
	}

	if _, _, err := failing.openai(context.Background(), nil, openaiInput{Prompt: "a cat"}); err == nil {
		t.Fatal("openai() error = nil, want an error")
	}

	for _, entry := range logs.All() {
		context := fmt.Sprint(entry.ContextMap())

		for _, secret := range []string{"sk-test", base64.StdEncoding.EncodeToString(image)} {
			if strings.Contains(entry.Message, secret) || strings.Contains(context, secret) {
				t.Errorf("log entry %q leaks %q", entry.Message, secret)
			}
		}
	}
}

func decodeInlineImage(t *testing.T, field, inline string) {
	t.Helper()

	payload, ok := strings.CutPrefix(inline, "data:image/png;base64,")
	if !ok {
		t.Fatalf("%s = %q, want a png data URL", field, inline)
	}

	if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
		t.Errorf("%s payload decode error: %v", field, err)
	}
}
