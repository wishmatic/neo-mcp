package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/present"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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

	image := base64.StdEncoding.EncodeToString(testImagePNG(t))

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

	frame := novelaiFrame(t, testImagePNG(t))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		_, _ = w.Write(frame)
	}))

	t.Cleanup(server.Close)

	return novelai.New(server.URL, "sk-test", false)
}

func newInitImageURL(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testImagePNG(t))
	}))

	t.Cleanup(server.Close)

	return server.URL + "/init.png"
}

func newResolver(t *testing.T) *resolve.Resolver {
	t.Helper()

	resolver, err := resolve.New(nil, "")
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	return resolver
}

func observedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)

	return zap.New(core), logs
}

func generationInputFor(model string) generationInput {
	return generationInput{
		Model:         model,
		Prompt:        "a cat",
		SamplingSteps: 20,
		Width:         512,
		Height:        512,
		CFGScale:      7,
		Seed:          -1,
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

func TestTxt2ImgCallToolDefaultsToUserAudience(t *testing.T) {
	backend := newNovelAIBackend(t, &requestLog{})

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: imagegen.New(nil, backend),
		Publisher: newTestPublisher(t),
		NovelAI:   backend,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "txt2img",
		Arguments: map[string]any{
			"model":  "nai-diffusion-5-full",
			"prompt": "a cat",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("omitting for_assistant was rejected: %+v", result.Content)
	}

	if len(result.Content) != 2 {
		t.Fatalf("content = %d, want a URL and one image", len(result.Content))
	}

	img, ok := result.Content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
	}

	if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, []mcp.Role{present.RoleUser}) {
		t.Errorf("audience = %+v, want [user]", img.Annotations)
	}
}

func TestTxt2ImgCallToolImageAudience(t *testing.T) {
	tests := map[bool][]mcp.Role{
		false: {present.RoleUser},
		true:  {present.RoleAssistant, present.RoleUser},
	}

	for forAssistant, want := range tests {
		t.Run(strconv.FormatBool(forAssistant), func(t *testing.T) {
			backend := newNovelAIBackend(t, &requestLog{})

			srv, err := New(Deps{
				Log:       zapNop(),
				Generator: imagegen.New(nil, backend),
				Publisher: newTestPublisher(t),
				NovelAI:   backend,
			})
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
				Name: "txt2img",
				Arguments: map[string]any{
					"model":         "nai-diffusion-5-full",
					"prompt":        "a cat",
					"for_assistant": forAssistant,
				},
			})
			if err != nil {
				t.Fatalf("CallTool() error: %v", err)
			}

			if result.IsError {
				t.Fatalf("CallTool() tool error: %+v", result.Content)
			}

			if len(result.Content) != 2 {
				t.Fatalf("content = %d, want a URL and one image", len(result.Content))
			}

			if _, ok := result.Content[0].(*mcp.TextContent); !ok {
				t.Fatalf("content[0] = %#v, want the URL as text", result.Content[0])
			}

			img, ok := result.Content[1].(*mcp.ImageContent)
			if !ok {
				t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
			}

			if img.MIMEType != "image/webp" {
				t.Errorf("mime type = %q, want image/webp", img.MIMEType)
			}

			if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, want) {
				t.Errorf("audience = %+v, want %v", img.Annotations, want)
			}

			if _, format, err := image.Decode(bytes.NewReader(img.Data)); err != nil || format != "webp" {
				t.Errorf("decode inline image = %q, %v, want webp", format, err)
			}
		})
	}
}

func TestTxt2ImgCallToolNovelAIWithDefaults(t *testing.T) {
	novelaiLog := &requestLog{}
	backend := newNovelAIBackend(t, novelaiLog)

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: imagegen.New(nil, backend),
		Publisher: newTestPublisher(t),
		NovelAI:   backend,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "txt2img",
		Arguments: map[string]any{
			"model":  "nai-diffusion-5-full",
			"prompt": "a cat",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	_, bodies := novelaiLog.snapshot()
	if len(bodies) != 1 {
		t.Fatalf("requests = %d, want 1", len(bodies))
	}

	params := paramsOf(t, decodeJSONBody(t, bodies[0]))

	if got := numberField(t, params, "width"); got != 512 {
		t.Errorf("width = %v, want the schema default 512", got)
	}

	if got := numberField(t, params, "steps"); got != 20 {
		t.Errorf("steps = %v, want the schema default 20", got)
	}

	if params["sampler"] != "k_euler_ancestral" {
		t.Errorf("sampler = %v, want the NovelAI default", params["sampler"])
	}
}

func TestNovelAILogsDoNotLeakSecrets(t *testing.T) {
	initImage := []byte("\x89PNG\r\n\x1a\n")
	initBase64 := base64.StdEncoding.EncodeToString(initImage)

	novelaiLog := &requestLog{}
	log, logs := observedLogger()

	h := &handlers{
		log:       log,
		gen:       imagegen.New(nil, newNovelAIBackend(t, novelaiLog)),
		publisher: newTestPublisher(t),
		resolver:  newResolver(t),
	}

	in := img2imgInput{
		generationInput:   generationInputFor("nai-diffusion-5-full"),
		InitImageURL:      newInitImageURL(t),
		DenoisingStrength: 0.6,
	}

	if _, _, err := h.img2img(context.Background(), nil, in); err != nil {
		t.Fatalf("img2img() error: %v", err)
	}

	failing := &handlers{
		log:       log,
		gen:       imagegen.New(nil, novelai.New("http://127.0.0.1:1", "sk-test", false)),
		publisher: newTestPublisher(t),
	}

	if _, _, err := failing.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInputFor("nai-diffusion-5-full"),
	}); err == nil {
		t.Fatal("txt2img() error = nil, want an error")
	}

	for _, entry := range logs.All() {
		context := fmt.Sprint(entry.ContextMap())

		for _, secret := range []string{"sk-test", initBase64} {
			if strings.Contains(entry.Message, secret) || strings.Contains(context, secret) {
				t.Errorf("log entry %q leaks %q", entry.Message, secret)
			}
		}
	}
}
