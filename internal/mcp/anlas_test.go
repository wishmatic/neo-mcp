package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/novelai"
)

func newNovelAIAccountBackend(t *testing.T, body string) *novelai.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return novelai.New(server.URL, "sk-test", false)
}

func TestAnlasTool(t *testing.T) {
	h := &handlers{
		log: zapNop(),
		novelai: newNovelAIAccountBackend(t,
			`{"trainingStepsLeft":{"fixedTrainingStepsLeft":9961,"purchasedTrainingSteps":32},"usage":{"percent":98}}`),
	}

	result, out, err := h.anlas(context.Background(), nil, anlasInput{})
	if err != nil {
		t.Fatalf("anlas() error: %v", err)
	}

	if out.Total != 9993 || out.Subscription != 9961 || out.Purchased != 32 {
		t.Errorf("out = %+v, want total 9993, subscription 9961, purchased 32", out)
	}

	if out.UsagePercent == nil || *out.UsagePercent != 98 {
		t.Errorf("usage percent = %v, want 98", out.UsagePercent)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", result.Content[0])
	}

	if !strings.Contains(text.Text, "9993") {
		t.Errorf("text = %q, want it to contain the total", text.Text)
	}
}

func TestAnlasToolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("nope"))
	}))

	t.Cleanup(server.Close)

	h := &handlers{log: zapNop(), novelai: novelai.New(server.URL, "sk-test", false)}

	if _, _, err := h.anlas(context.Background(), nil, anlasInput{}); err == nil {
		t.Fatal("anlas() error = nil, want an error")
	}
}

func TestAnlasCallToolWithoutInput(t *testing.T) {
	srv, err := New(Deps{
		Log:     zapNop(),
		NovelAI: newNovelAIAccountBackend(t, `{"trainingStepsLeft":{"fixedTrainingStepsLeft":7}}`),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{Name: "anlas"})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map[string]any", result.StructuredContent)
	}

	if structured["total"] != float64(7) {
		t.Errorf("structured content = %+v, want total 7", structured)
	}
}
