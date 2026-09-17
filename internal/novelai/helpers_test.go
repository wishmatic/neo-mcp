package novelai

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

type generateCapture struct {
	path          string
	authorization string
	contentType   string
	body          []byte
}

func encodeFrame(t *testing.T, event map[string]any) []byte {
	t.Helper()

	payload, err := msgpack.Marshal(event)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}

	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)

	return frame
}

func newGenerateServer(t *testing.T, frames ...[]byte) (*httptest.Server, *generateCapture) {
	t.Helper()

	captured := &generateCapture{}
	body := bytes.Join(frames, nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.authorization = r.Header.Get("Authorization")
		captured.contentType = r.Header.Get("Content-Type")
		captured.body, _ = io.ReadAll(r.Body)
		_, _ = w.Write(body)
	}))

	t.Cleanup(server.Close)

	return server, captured
}

func decodeJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	return decoded
}

func nestedMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %T, want a map", key, parent[key])
	}

	return value
}

func asFloat(t *testing.T, parent map[string]any, key string) float64 {
	t.Helper()

	value, ok := parent[key].(float64)
	if !ok {
		t.Fatalf("%s = %T, want a number", key, parent[key])
	}

	return value
}

func marshalParams(t *testing.T, params map[string]any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	return decodeJSON(t, raw)
}
