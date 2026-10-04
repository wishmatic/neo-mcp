package openai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type capturedRequest struct {
	path          string
	authorization string
	contentType   string
	body          []byte
}

func newImageServer(t *testing.T, status int, response string) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.authorization = r.Header.Get("Authorization")
		captured.contentType = r.Header.Get("Content-Type")
		captured.body, _ = io.ReadAll(r.Body)

		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(server.Close)

	return server, captured
}

func testProvider(t *testing.T, baseURL string) Provider {
	t.Helper()

	return Provider{Name: "primary", BaseURL: baseURL, APIKey: "sk-test"}
}

func bodyOf(t *testing.T, captured *capturedRequest) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(captured.body, &body); err != nil {
		t.Fatalf("decode request body %q: %v", captured.body, err)
	}

	return body
}
