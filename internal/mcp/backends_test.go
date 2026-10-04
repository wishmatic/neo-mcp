package mcp

import (
	"encoding/base64"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/openai"
)

type requestLog struct {
	mu     sync.Mutex
	paths  []string
	bodies [][]byte
}

func (l *requestLog) add(req *http.Request) {
	body, _ := io.ReadAll(req.Body)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.paths = append(l.paths, req.URL.Path)
	l.bodies = append(l.bodies, body)
}

func (l *requestLog) snapshot() ([]string, [][]byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.paths...), append([][]byte(nil), l.bodies...)
}

func newForgeBackend(t *testing.T, log *requestLog) *forge.Client {
	t.Helper()

	image := base64.StdEncoding.EncodeToString(testImagePNG(t))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["` + image + `"]}`))
	}))

	t.Cleanup(server.Close)

	return forge.New(server.URL, zapNop())
}

func newInitImageURL(t *testing.T) string {
	t.Helper()

	return newSizedInitImageURL(t, testImageSize, testImageSize)
}

func newOpenAIBackend(t *testing.T, log *requestLog) *openai.Client {
	t.Helper()

	return openai.New([]openai.Provider{newOpenAIProvider(t, log, openaiInlineResponse(t, testImagePNG(t)))})
}

func newOpenAIProvider(t *testing.T, log *requestLog, response string) openai.Provider {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(server.Close)

	return openai.Provider{Name: "primary", BaseURL: server.URL, APIKey: "sk-test"}
}

func openaiInlineResponse(t *testing.T, image []byte) string {
	t.Helper()

	return `{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(image) + `"}],"cost":0.003}`
}

func newSizedInitImageURL(t *testing.T, width, height int) string {
	t.Helper()

	data, err := format.Encode(image.NewNRGBA(image.Rect(0, 0, width, height)), format.PNG)
	if err != nil {
		t.Fatalf("encode init image: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))

	t.Cleanup(server.Close)

	return server.URL + "/init.png"
}
