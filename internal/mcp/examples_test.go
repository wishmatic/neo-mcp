package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const examplesModel = "sd_xl_base_1.0.safetensors"

func newTestStore(t *testing.T) *store.Client {
	t.Helper()

	client, err := store.New(filepath.Join(t.TempDir(), "neo.db"))
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	return client
}

func textContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", result.Content[0])
	}

	return text.Text
}

func newExamplesSession(
	t *testing.T,
	client *store.Client,
	store publish.Store,
	enabled bool,
) *mcp.ClientSession {
	t.Helper()

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: imagegen.New(newForgeBackend(t, &requestLog{}), nil),
		Publisher: publish.New(store, zapNop()),
		Resolver:  newResolver(t),
		Store:     client,
		Examples:  ExamplesConfig{Enabled: enabled},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return connectSession(t, srv)
}

func callTxt2Img(t *testing.T, session *mcp.ClientSession, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	if args == nil {
		args = map[string]any{
			"model":           examplesModel,
			"prompt":          "a cat on a fence",
			"negative_prompt": "blurry",
			"steps":           25,
			"width":           768,
			"height":          512,
			"cfg_scale":       6.5,
			"seed":            1234,
			"return_as":       "url",
		}
	}

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "txt2img", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(txt2img) error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool(txt2img) tool error: %+v", result.Content)
	}

	return result
}

func callImg2Img(t *testing.T, session *mcp.ClientSession, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	if args == nil {
		args = map[string]any{
			"model":              examplesModel,
			"prompt":             "a dog on a beach",
			"init_image_url":     newInitImageURL(t),
			"denoising_strength": 0.6,
			"noise":              0.1,
			"sampler_name":       "DPM++ 2M",
			"return_as":          "url",
		}
	}

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "img2img", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(img2img) error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool(img2img) tool error: %+v", result.Content)
	}

	return result
}

func resultURL(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", result.StructuredContent)
	}

	urls, ok := structured["urls"].([]any)
	if !ok || len(urls) != 1 {
		t.Fatalf("urls = %v, want exactly one", structured["urls"])
	}

	url, ok := urls[0].(string)
	if !ok || !strings.HasPrefix(url, "https://cdn.example.com/") {
		t.Fatalf("url = %v, want an uploaded URL", urls[0])
	}

	return url
}

func storedQuery(t *testing.T, example store.Example) map[string]any {
	t.Helper()

	var query map[string]any
	if err := json.Unmarshal([]byte(example.Query), &query); err != nil {
		t.Fatalf("query %q is not JSON: %v", example.Query, err)
	}

	return query
}

func TestSaveExamplesCaptureTxt2Img(t *testing.T) {
	client := newTestStore(t)
	session := newExamplesSession(t, client, newFakeStore(t), true)
	ctx := context.Background()

	result := callTxt2Img(t, session, nil)
	url := resultURL(t, result)

	examples, err := client.RandomExamples(ctx, examplesModel, 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 {
		t.Fatalf("examples = %d, want 1", len(examples))
	}

	example := examples[0]

	if example.Model != examplesModel || example.Tool != "txt2img" {
		t.Errorf("example = %+v, want the txt2img call", example)
	}

	if example.URL != url {
		t.Errorf("url = %q, want the URL the tool returned (%q)", example.URL, url)
	}

	query := storedQuery(t, example)

	want := map[string]any{
		"model":           examplesModel,
		"prompt":          "a cat on a fence",
		"negative_prompt": "blurry",
		"steps":           float64(25),
		"width":           float64(768),
		"height":          float64(512),
		"cfg_scale":       6.5,
		"seed":            float64(1234),
	}

	for field, value := range want {
		if query[field] != value {
			t.Errorf("query[%s] = %v, want %v", field, query[field], value)
		}
	}

	for _, field := range []string{"sampler_name", "scheduler"} {
		if _, ok := query[field]; ok {
			t.Errorf("query[%s] = %v, want it absent when unset", field, query[field])
		}
	}
}

func TestSaveExamplesCaptureImg2Img(t *testing.T) {
	client := newTestStore(t)
	session := newExamplesSession(t, client, newFakeStore(t), true)
	ctx := context.Background()

	result := callImg2Img(t, session, nil)
	url := resultURL(t, result)

	examples, err := client.RandomExamples(ctx, examplesModel, 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 {
		t.Fatalf("examples = %d, want 1", len(examples))
	}

	example := examples[0]

	if example.Tool != "img2img" {
		t.Errorf("tool = %q, want img2img", example.Tool)
	}

	if example.URL != url {
		t.Errorf("url = %q, want %q", example.URL, url)
	}

	query := storedQuery(t, example)

	if query["denoising_strength"] != 0.6 || query["noise"] != 0.1 {
		t.Errorf("query = %v, want the img2img settings", query)
	}

	if query["sampler_name"] != "DPM++ 2M" {
		t.Errorf("query[sampler_name] = %v, want the requested sampler", query["sampler_name"])
	}

	initURL, ok := query["init_image_url"].(string)
	if !ok || !strings.Contains(initURL, "/init.png") {
		t.Errorf("query[init_image_url] = %v, want the init image URL", query["init_image_url"])
	}
}

func TestSaveExamplesDisabled(t *testing.T) {
	client := newTestStore(t)
	session := newExamplesSession(t, client, newFakeStore(t), false)
	ctx := context.Background()

	callTxt2Img(t, session, nil)

	examples, err := client.RandomExamples(ctx, examplesModel, 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 0 {
		t.Fatalf("examples = %d, want none when the feature is disabled", len(examples))
	}
}

func TestSaveExamplesFailureIsBestEffort(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	client := newTestStore(t)

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	h := &handlers{
		log:            zap.New(core),
		gen:            imagegen.New(newForgeBackend(t, &requestLog{}), nil),
		publisher:      publish.New(newFakeStore(t), zapNop()),
		store:          client,
		examplesConfig: ExamplesConfig{Enabled: true},
	}

	result, out, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInput{Model: examplesModel, Prompt: "a cat on a fence"},
	})
	if err != nil {
		t.Fatalf("txt2img() error: %v, want the capture failure to be swallowed", err)
	}

	if out.Count != 1 || len(out.URLs) != 1 {
		t.Fatalf("output = %+v, want the generated image returned", out)
	}

	if _, ok := result.Content[0].(*mcp.TextContent); !ok {
		t.Fatalf("content type = %T, want a URL", result.Content[0])
	}

	warned := false

	for _, entry := range logs.All() {
		context := fmt.Sprint(entry.ContextMap())

		if strings.Contains(entry.Message, "a cat on a fence") || strings.Contains(context, "a cat on a fence") {
			t.Error("a log entry leaks the prompt")
		}

		if strings.Contains(entry.Message, out.URLs[0]) || strings.Contains(context, out.URLs[0]) {
			t.Error("a log entry leaks the URL")
		}

		if entry.Level == zapcore.WarnLevel && strings.Contains(entry.Message, "examples") {
			warned = true
		}
	}

	if !warned {
		t.Error("no warning was logged for the failed capture")
	}
}

func TestSaveExamplesCancelledContext(t *testing.T) {
	client := newTestStore(t)
	h := &handlers{
		log:            zapNop(),
		store:          client,
		examplesConfig: ExamplesConfig{Enabled: true},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h.saveExamples(ctx, "txt2img", examplesModel, false, txt2imgInput{
		generationInput: generationInput{Model: examplesModel, Prompt: "a cat"},
	}, []string{"https://cdn.example.com/a.png"})

	examples, err := client.RandomExamples(context.Background(), examplesModel, 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 0 {
		t.Fatalf("examples = %d, want none for a cancelled context", len(examples))
	}
}

func TestSaveExamplesCaptureNovelAIModel(t *testing.T) {
	client := newTestStore(t)
	h := &handlers{
		log:            zapNop(),
		gen:            imagegen.New(nil, newNovelAIBackend(t, &requestLog{})),
		publisher:      publish.New(newFakeStore(t), zapNop()),
		store:          client,
		examplesConfig: ExamplesConfig{Enabled: true},
	}

	_, out, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInput{Model: "nai-diffusion-5-full", Prompt: "a cat", Width: 512, Height: 512},
	})
	if err != nil {
		t.Fatalf("txt2img() error: %v", err)
	}

	if len(out.URLs) != 1 {
		t.Fatalf("urls = %v, want one", out.URLs)
	}

	examples, err := client.RandomExamples(context.Background(), "nai-diffusion-5-full", 1)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 || examples[0].Model != "nai-diffusion-5-full" {
		t.Fatalf("examples = %+v, want the NovelAI model as the key", examples)
	}
}

func TestSaveExamplesUploadFailureWritesNothing(t *testing.T) {
	client := newTestStore(t)
	h := &handlers{
		log:            zapNop(),
		gen:            imagegen.New(newForgeBackend(t, &requestLog{}), nil),
		publisher:      publish.New(newFailingStore(t), zapNop()),
		store:          client,
		examplesConfig: ExamplesConfig{Enabled: true},
	}

	_, _, err := h.txt2img(context.Background(), nil, txt2imgInput{
		generationInput: generationInput{Model: examplesModel, Prompt: "a cat"},
	})
	if err == nil {
		t.Fatal("txt2img() error = nil, want the upload failure")
	}

	examples, err := client.RandomExamples(context.Background(), examplesModel, 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 0 {
		t.Fatalf("examples = %d, want none after a failed upload", len(examples))
	}
}

func TestExamplesRegistration(t *testing.T) {
	tests := []struct {
		name     string
		store    bool
		uploader bool
		enabled  bool
		want     bool
	}{
		{name: "enabled with store and uploader", store: true, uploader: true, enabled: true, want: true},
		{name: "disabled", store: true, uploader: true, want: false},
		{name: "no uploader", store: true, enabled: true, want: false},
		{name: "no store", uploader: true, enabled: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := Deps{Log: zapNop(), Examples: ExamplesConfig{Enabled: tt.enabled}}

			if tt.store {
				deps.Store = newTestStore(t)
			}

			if tt.uploader {
				deps.Publisher = publish.New(newFakeStore(t), zapNop())
			}

			srv, err := New(deps)
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			if got := slices.Contains(toolNames(t, srv), "examples"); got != tt.want {
				t.Errorf("examples registered = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExamplesSchema(t *testing.T) {
	s := examplesSchema()

	if !slices.Contains(s.Required, "model") {
		t.Error("model is not required")
	}

	if s.Properties["model"].Default != nil {
		t.Error("model must not have a default")
	}

	n := s.Properties["n"]
	if n == nil {
		t.Fatal("n property is missing")
	}

	if slices.Contains(s.Required, "n") {
		t.Error("n must not be required")
	}

	if n.Minimum == nil || *n.Minimum != 1 {
		t.Errorf("n minimum = %v, want 1", n.Minimum)
	}

	if string(n.Default) != "2" {
		t.Errorf("n default = %s, want 2", n.Default)
	}

	nsfw := s.Properties["nsfw"]
	if nsfw == nil {
		t.Fatal("nsfw property is missing")
	}

	if slices.Contains(s.Required, "nsfw") {
		t.Error("nsfw must not be required")
	}

	if nsfw.Minimum == nil || *nsfw.Minimum != -1 {
		t.Errorf("nsfw minimum = %v, want -1", nsfw.Minimum)
	}

	if nsfw.Maximum == nil || *nsfw.Maximum != 1 {
		t.Errorf("nsfw maximum = %v, want 1", nsfw.Maximum)
	}

	if string(nsfw.Default) != "0" {
		t.Errorf("nsfw default = %s, want 0", nsfw.Default)
	}

	if len(s.Properties) != 3 {
		t.Errorf("properties = %v, want model, n, and nsfw", s.Properties)
	}
}

func TestExamplesCallTool(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()

	session := newExamplesSession(t, client, newFakeStore(t), true)

	empty, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "examples",
		Arguments: map[string]any{"model": examplesModel},
	})
	if err != nil {
		t.Fatalf("CallTool(examples) error: %v", err)
	}

	if empty.IsError {
		t.Fatalf("CallTool(examples) tool error: %+v", empty.Content)
	}

	if text := textContent(t, empty); text != "No examples saved for "+examplesModel+" yet." {
		t.Errorf("text = %q, want the empty message", text)
	}

	structured, ok := empty.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", empty.StructuredContent)
	}

	if structured["count"] != float64(0) || structured["model"] != examplesModel {
		t.Errorf("structured content = %+v, want count 0", structured)
	}

	result := callTxt2Img(t, session, nil)
	url := resultURL(t, result)

	got, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "examples",
		Arguments: map[string]any{"model": examplesModel},
	})
	if err != nil {
		t.Fatalf("CallTool(examples) error: %v", err)
	}

	if got.IsError {
		t.Fatalf("CallTool(examples) tool error: %+v", got.Content)
	}

	text := textContent(t, got)

	for _, want := range []string{
		"# Examples for " + examplesModel,
		"## Example 1 (txt2img)",
		"- Image: " + url,
		`"prompt": "a cat on a fence"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text =\n%s\nwant it to contain %q", text, want)
		}
	}

	structured, ok = got.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", got.StructuredContent)
	}

	if structured["count"] != float64(1) || structured["model"] != examplesModel {
		t.Errorf("structured content = %+v, want count 1", structured)
	}
}

func TestExamplesStoreError(t *testing.T) {
	client := newTestStore(t)

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	h := &handlers{
		log:            zapNop(),
		store:          client,
		examplesConfig: ExamplesConfig{Enabled: true},
	}

	_, _, err := h.examples(context.Background(), nil, examplesInput{Model: "m"})
	if err == nil {
		t.Fatal("examples() error = nil, want a store error")
	}

	if !strings.HasPrefix(err.Error(), "examples:") {
		t.Errorf("error = %q, want an examples prefix", err.Error())
	}
}

func TestExamplesHonoursCount(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		if _, err := client.SaveExample(ctx, store.ExampleMeta{
			Model: examplesModel,
			Tool:  "txt2img",
			Query: `{"prompt":"a cat"}`,
			URL:   fmt.Sprintf("https://cdn.example.com/%d.png", i),
		}); err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}
	}

	session := newExamplesSession(t, client, newFakeStore(t), true)

	for _, tt := range []struct {
		name string
		args map[string]any
		want int
	}{
		{name: "default", args: map[string]any{"model": examplesModel}, want: defaultExampleCount},
		{name: "explicit", args: map[string]any{"model": examplesModel, "n": 3}, want: 3},
		{name: "one", args: map[string]any{"model": examplesModel, "n": 1}, want: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "examples", Arguments: tt.args})
			if err != nil {
				t.Fatalf("CallTool(examples) error: %v", err)
			}

			if result.IsError {
				t.Fatalf("CallTool(examples) tool error: %+v", result.Content)
			}

			structured, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("structured content = %T, want map[string]any", result.StructuredContent)
			}

			if structured["count"] != float64(tt.want) {
				t.Errorf("count = %v, want %d", structured["count"], tt.want)
			}
		})
	}
}

func TestSaveExamplesNSFWFlag(t *testing.T) {
	tests := []struct {
		name string
		nsfw bool
		want bool
	}{
		{name: "flagged", nsfw: true, want: true},
		{name: "not flagged", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestStore(t)
			h := &handlers{
				log:            zapNop(),
				gen:            imagegen.New(newForgeBackend(t, &requestLog{}), nil),
				publisher:      publish.New(newFakeStore(t), zapNop()),
				store:          client,
				examplesConfig: ExamplesConfig{Enabled: true},
			}

			_, out, err := h.txt2img(context.Background(), nil, txt2imgInput{
				generationInput: generationInput{Model: examplesModel, Prompt: "a cat on a fence", NSFW: tt.nsfw},
			})
			if err != nil {
				t.Fatalf("txt2img() error: %v", err)
			}

			if len(out.URLs) != 1 {
				t.Fatalf("urls = %v, want 1", out.URLs)
			}

			examples, err := client.RandomExamples(context.Background(), examplesModel, 1)
			if err != nil {
				t.Fatalf("RandomExamples() error: %v", err)
			}

			if len(examples) != 1 {
				t.Fatalf("examples = %d, want 1", len(examples))
			}

			if examples[0].NSFW != tt.want {
				t.Errorf("NSFW = %v, want %v", examples[0].NSFW, tt.want)
			}
		})
	}
}

func TestExamplesNSFWFilter(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()

	for i, nsfw := range []bool{false, false, true} {
		meta := store.ExampleMeta{
			Model: examplesModel,
			Tool:  "txt2img",
			Query: `{"prompt":"a cat"}`,
			URL:   fmt.Sprintf("https://cdn.example.com/%d.png", i),
			NSFW:  nsfw,
		}

		if _, err := client.SaveExample(ctx, meta); err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}
	}

	session := newExamplesSession(t, client, newFakeStore(t), true)

	for _, tt := range []struct {
		name string
		args map[string]any
		want int
	}{
		{name: "any", args: map[string]any{"model": examplesModel, "n": 3}, want: 3},
		{name: "non-nsfw", args: map[string]any{"model": examplesModel, "nsfw": -1}, want: 2},
		{name: "nsfw", args: map[string]any{"model": examplesModel, "nsfw": 1}, want: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "examples", Arguments: tt.args})
			if err != nil {
				t.Fatalf("CallTool(examples) error: %v", err)
			}

			if result.IsError {
				t.Fatalf("CallTool(examples) tool error: %+v", result.Content)
			}

			structured, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("structured content = %T, want map[string]any", result.StructuredContent)
			}

			if structured["count"] != float64(tt.want) {
				t.Errorf("count = %v, want %d", structured["count"], tt.want)
			}
		})
	}

	_, _, err := (&handlers{log: zapNop(), store: client, examplesConfig: ExamplesConfig{Enabled: true}}).examples(
		ctx, nil, examplesInput{Model: examplesModel, NSFW: 2},
	)
	if err == nil {
		t.Error("examples(nsfw=2) error = nil, want an error")
	}
}
