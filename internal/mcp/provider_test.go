package mcp

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/wishmatic/neo-mcp/internal/novelai"
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

func newInitImageURL(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
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

func TestTxt2ImgRoutesNovelAIModel(t *testing.T) {
	forgeLog := &requestLog{}
	novelaiLog := &requestLog{}

	h := &handlers{
		log:     zapNop(),
		forge:   newForgeBackend(t, forgeLog),
		novelai: newNovelAIBackend(t, novelaiLog),
	}

	result, out, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInputFor("nai-diffusion-5-full"),
	})
	if err != nil {
		t.Fatalf("txt2img() error: %v", err)
	}

	if out.Count != 1 {
		t.Errorf("count = %d, want 1", out.Count)
	}

	if len(out.URLs) != 0 {
		t.Errorf("urls = %v, want none without S3", out.URLs)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	image, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.ImageContent", result.Content[0])
	}

	if string(image.Data) != "novelai-png" || image.MIMEType != "image/png" {
		t.Errorf("image = %q (%s), want novelai-png (image/png)", image.Data, image.MIMEType)
	}

	if len(out.URLs) != 0 {
		t.Errorf("urls = %v, want none without S3", out.URLs)
	}

	if paths, _ := forgeLog.snapshot(); len(paths) != 0 {
		t.Errorf("forge received %v, want no requests", paths)
	}

	paths, bodies := novelaiLog.snapshot()
	if len(paths) != 1 || paths[0] != "/ai/generate-image-stream" {
		t.Fatalf("novelai received %v, want one stream request", paths)
	}

	if body := decodeJSONBody(t, bodies[0]); body["model"] != "nai-diffusion-5-full" {
		t.Errorf("model = %v, want nai-diffusion-5-full", body["model"])
	}
}

func TestTxt2ImgRoutesForgeModel(t *testing.T) {
	forgeLog := &requestLog{}
	novelaiLog := &requestLog{}

	h := &handlers{
		log:     zapNop(),
		forge:   newForgeBackend(t, forgeLog),
		novelai: newNovelAIBackend(t, novelaiLog),
	}

	_, _, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInputFor("sd_xl_base_1.0.safetensors"),
	})
	if err != nil {
		t.Fatalf("txt2img() error: %v", err)
	}

	if paths, _ := novelaiLog.snapshot(); len(paths) != 0 {
		t.Errorf("novelai received %v, want no requests", paths)
	}

	paths, _ := forgeLog.snapshot()
	if len(paths) != 1 || paths[0] != "/sdapi/v1/txt2img" {
		t.Fatalf("forge received %v, want one txt2img request", paths)
	}
}

func TestTxt2ImgNovelAIWithoutClient(t *testing.T) {
	h := &handlers{log: zapNop()}

	_, _, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInputFor("nai-diffusion-5-full"),
	})
	if err == nil {
		t.Fatal("txt2img() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "NOVELAI_API_KEY") {
		t.Errorf("error = %q, want it to name NOVELAI_API_KEY", err.Error())
	}
}

func TestTxt2ImgNovelAIIgnoresForgeOnlyFields(t *testing.T) {
	novelaiLog := &requestLog{}

	h := &handlers{log: zapNop(), novelai: newNovelAIBackend(t, novelaiLog)}

	in := txt2imgInput{
		generationInput:   generationInputFor("nai-diffusion-5-full"),
		DenoisingStrength: 0.5,
	}
	in.ForgePreset = "krea"
	in.VAEAndTextModels = []string{"vae.safetensors"}
	in.ScheduleType = "Automatic"
	in.EnableHR = true
	in.HRScale = 2
	in.HRUpscaler = "4x-AnimeSharp"
	in.HRSecondPassSteps = 10
	in.HRCFGScale = 5

	if _, _, err := h.txt2img(context.Background(), nil, in); err != nil {
		t.Fatalf("txt2img() error: %v", err)
	}

	_, bodies := novelaiLog.snapshot()
	body := decodeJSONBody(t, bodies[0])
	params := paramsOf(t, body)

	dropped := []string{
		"forge_preset", "vae_and_text_models", "scheduler", "enable_hr",
		"hr_scale", "hr_upscaler", "hr_second_pass_steps", "hr_cfg",
		"denoising_strength", "strength",
	}

	for _, key := range []string{"forge_preset", "scheduler", "enable_hr"} {
		if _, ok := body[key]; ok {
			t.Errorf("%s is present at the top level, want it dropped", key)
		}
	}

	for _, key := range dropped {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present in the NovelAI payload, want it dropped", key)
		}
	}
}

func TestTxt2ImgNovelAISampler(t *testing.T) {
	tests := []struct {
		name    string
		sampler string
		want    string
	}{
		{name: "empty uses the NovelAI default", sampler: "", want: "k_euler_ancestral"},
		{name: "known sampler passes through", sampler: "k_dpmpp_2m", want: "k_dpmpp_2m"},
		{name: "unknown sampler passes through", sampler: "some_future_sampler", want: "some_future_sampler"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			novelaiLog := &requestLog{}
			h := &handlers{log: zapNop(), novelai: newNovelAIBackend(t, novelaiLog)}

			in := txt2imgInput{generationInput: generationInputFor("nai-diffusion-5-full")}
			in.SamplingMethod = tt.sampler

			if _, _, err := h.txt2img(context.Background(), nil, in); err != nil {
				t.Fatalf("txt2img() error: %v", err)
			}

			_, bodies := novelaiLog.snapshot()
			params := paramsOf(t, decodeJSONBody(t, bodies[0]))

			if params["sampler"] != tt.want {
				t.Errorf("sampler = %v, want %s", params["sampler"], tt.want)
			}
		})
	}
}

func TestTxt2ImgForgeSamplerDefaults(t *testing.T) {
	forgeLog := &requestLog{}

	h := &handlers{log: zapNop(), forge: newForgeBackend(t, forgeLog)}

	if _, _, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInputFor("sd_xl_base_1.0.safetensors"),
	}); err != nil {
		t.Fatalf("txt2img() error: %v", err)
	}

	_, bodies := forgeLog.snapshot()
	body := decodeJSONBody(t, bodies[0])

	if body["sampler_name"] != "DPM++ 2M" {
		t.Errorf("sampler_name = %v, want DPM++ 2M", body["sampler_name"])
	}

	if body["scheduler"] != "Automatic" {
		t.Errorf("scheduler = %v, want Automatic", body["scheduler"])
	}
}

func TestTxt2ImgCallToolNovelAIWithDefaults(t *testing.T) {
	novelaiLog := &requestLog{}

	srv, err := New(zapNop(), nil, newNovelAIBackend(t, novelaiLog), nil, nil, nil, nil)
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

func TestImg2ImgNovelAIMapsStrengthAndNoise(t *testing.T) {
	novelaiLog := &requestLog{}

	h := &handlers{
		log:      zapNop(),
		novelai:  newNovelAIBackend(t, novelaiLog),
		resolver: newResolver(t),
	}

	in := img2imgInput{
		generationInput:   generationInputFor("nai-diffusion-5-full"),
		InitImageURL:      newInitImageURL(t),
		DenoisingStrength: 0.6,
		Noise:             0.1,
	}
	in.ScheduleType = "Automatic"
	in.EnableHR = true

	if _, _, err := h.img2img(context.Background(), nil, in); err != nil {
		t.Fatalf("img2img() error: %v", err)
	}

	_, bodies := novelaiLog.snapshot()
	body := decodeJSONBody(t, bodies[0])

	if body["action"] != "img2img" {
		t.Errorf("action = %v, want img2img", body["action"])
	}

	params := paramsOf(t, body)

	if got := numberField(t, params, "strength"); got != 0.6 {
		t.Errorf("strength = %v, want 0.6", got)
	}

	if got := numberField(t, params, "noise"); got != 0.1 {
		t.Errorf("noise = %v, want 0.1", got)
	}

	if params["image"] != base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n")) {
		t.Errorf("image = %v, want the raw base64 init image", params["image"])
	}

	for _, key := range []string{"scheduler", "enable_hr"} {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present in the NovelAI payload, want it dropped", key)
		}
	}
}

func TestImg2ImgForgeIgnoresNoise(t *testing.T) {
	forgeLog := &requestLog{}

	h := &handlers{
		log:      zapNop(),
		forge:    newForgeBackend(t, forgeLog),
		resolver: newResolver(t),
	}

	if _, _, err := h.img2img(context.Background(), nil, img2imgInput{
		generationInput: generationInputFor("sd_xl_base_1.0.safetensors"),
		InitImageURL:    newInitImageURL(t),
		Noise:           0.1,
	}); err != nil {
		t.Fatalf("img2img() error: %v", err)
	}

	_, bodies := forgeLog.snapshot()
	if body := decodeJSONBody(t, bodies[0]); body["noise"] != nil {
		t.Errorf("noise = %v, want it dropped", body["noise"])
	}
}

func TestNovelAILogsDoNotLeakSecrets(t *testing.T) {
	initImage := []byte("\x89PNG\r\n\x1a\n")
	initBase64 := base64.StdEncoding.EncodeToString(initImage)

	novelaiLog := &requestLog{}
	log, logs := observedLogger()

	h := &handlers{
		log:      log,
		novelai:  newNovelAIBackend(t, novelaiLog),
		resolver: newResolver(t),
	}

	in := img2imgInput{
		generationInput:   generationInputFor("nai-diffusion-5-full"),
		InitImageURL:      newInitImageURL(t),
		DenoisingStrength: 0.6,
	}

	if _, _, err := h.img2img(context.Background(), nil, in); err != nil {
		t.Fatalf("img2img() error: %v", err)
	}

	failingLog := &requestLog{}
	failing := &handlers{
		log:      log,
		novelai:  newNovelAIBackend(t, failingLog),
		resolver: newResolver(t),
	}
	failing.novelai = novelai.New("http://127.0.0.1:1", "sk-test", false)

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
