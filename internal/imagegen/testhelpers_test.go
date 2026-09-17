package imagegen

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
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

func novelaiFrame(t *testing.T, image []byte) []byte {
	t.Helper()

	payload, err := msgpack.Marshal(map[string]any{"event_type": "final", "image": image})
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}

	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)

	return frame
}

func newForgeBackend(t *testing.T, log *requestLog) *sdwebui.Client {
	t.Helper()

	image := base64.StdEncoding.EncodeToString([]byte("forge-png"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["` + image + `"]}`))
	}))

	t.Cleanup(server.Close)

	return sdwebui.New(server.URL, false)
}

func newNovelAIBackend(t *testing.T, log *requestLog) *novelai.Client {
	t.Helper()

	frame := novelaiFrame(t, []byte("novelai-png"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		_, _ = w.Write(frame)
	}))

	t.Cleanup(server.Close)

	return novelai.New(server.URL, "sk-test", false)
}

func txt2ImgRequestFor(model string) Txt2ImgRequest {
	return Txt2ImgRequest{
		Params: Params{
			Model:    model,
			Prompt:   "a cat",
			Steps:    20,
			Width:    512,
			Height:   512,
			CFGScale: 7,
			Seed:     -1,
		},
	}
}

func decodeJSONBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	return decoded
}

func paramsOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	params, ok := body["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("parameters = %T, want a map", body["parameters"])
	}

	return params
}

func numberField(t *testing.T, params map[string]any, key string) float64 {
	t.Helper()

	value, ok := params[key].(float64)
	if !ok {
		t.Fatalf("%s = %T, want a number", key, params[key])
	}

	return value
}
