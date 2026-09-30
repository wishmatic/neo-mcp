package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/present"
)

func newNovelAIServer(t *testing.T, log *requestLog) *mcp.Server {
	t.Helper()

	srv, err := New(Clients{
		Log:      zapNop(),
		NovelAI:  newNovelAIBackend(t, log),
		Store:    newTestStore(t),
		Resolver: newResolver(t),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return srv
}

func TestNovelAICallToolUsesGenerateWithoutAnInitImage(t *testing.T) {
	log := &requestLog{}
	srv := newNovelAIServer(t, log)

	callTool(t, srv, "novelai", map[string]any{
		"model":  "nai-diffusion-5-full",
		"prompt": "a cat",
	})

	if path := singleRequestPath(t, log); path != "/ai/generate-image-stream" {
		t.Errorf("path = %q, want /ai/generate-image-stream", path)
	}

	body := singleRequestBody(t, log)

	if body["action"] != "generate" {
		t.Errorf("action = %v, want generate", body["action"])
	}

	if body["model"] != "nai-diffusion-5-full" {
		t.Errorf("model = %v, want nai-diffusion-5-full", body["model"])
	}

	params := paramsOf(t, body)

	if got := numberField(t, params, "width"); got != 512 {
		t.Errorf("width = %v, want the 512 the description documents", got)
	}

	if got := numberField(t, params, "steps"); got != 20 {
		t.Errorf("steps = %v, want the schema default 20", got)
	}

	if params["sampler"] != novelai.DefaultSampler {
		t.Errorf("sampler = %v, want the NovelAI default %s", params["sampler"], novelai.DefaultSampler)
	}

	for _, key := range []string{"image", "strength", "noise"} {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present, want it dropped without an init image", key)
		}
	}
}

func TestNovelAICallToolPassesValuesThrough(t *testing.T) {
	log := &requestLog{}
	srv := newNovelAIServer(t, log)

	callTool(t, srv, "novelai", map[string]any{
		"model":           "nai-diffusion-5-full",
		"prompt":          "a cat",
		"negative_prompt": "bad",
		"sampler_name":    "k_dpmpp_2m",
		"steps":           30,
		"width":           832,
		"height":          1216,
		"cfg_scale":       5,
		"seed":            42,
	})

	body := singleRequestBody(t, log)
	params := paramsOf(t, body)

	if params["negative_prompt"] != "bad" || params["sampler"] != "k_dpmpp_2m" {
		t.Errorf("params = %v, want the sampler and the negative prompt copied", params)
	}

	for field, want := range map[string]float64{
		"steps":  30,
		"width":  832,
		"height": 1216,
		"scale":  5,
		"seed":   42,
	} {
		if got := numberField(t, params, field); got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}
}

func TestNovelAICallToolUsesImg2ImgWithAnInitImage(t *testing.T) {
	log := &requestLog{}
	srv := newNovelAIServer(t, log)

	callTool(t, srv, "novelai", map[string]any{
		"model":              "nai-diffusion-5-full",
		"prompt":             "a cat",
		"init_image_url":     newSizedInitImageURL(t, 320, 640),
		"denoising_strength": 0.6,
		"noise":              0.1,
	})

	body := singleRequestBody(t, log)

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

	encoded, ok := params["image"].(string)
	if !ok || encoded == "" {
		t.Fatalf("image = %v, want raw base64", params["image"])
	}

	if strings.HasPrefix(encoded, "data:") {
		t.Errorf("image = %q, want no data URI prefix", encoded)
	}

	if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
		t.Errorf("image decode error: %v", err)
	}

	width, height := numberField(t, params, "width"), numberField(t, params, "height")
	if width != 320 || height != 640 {
		t.Errorf("dimensions = %vx%v, want the init image's 320x640", width, height)
	}
}

func TestNovelAICallToolDefaultsImg2ImgStrength(t *testing.T) {
	log := &requestLog{}
	srv := newNovelAIServer(t, log)

	callTool(t, srv, "novelai", map[string]any{
		"model":          "nai-diffusion-5-full",
		"prompt":         "a cat",
		"init_image_url": newInitImageURL(t),
	})

	params := paramsOf(t, singleRequestBody(t, log))

	if got := numberField(t, params, "strength"); got != 0.75 {
		t.Errorf("strength = %v, want the 0.75 the description documents", got)
	}
}

func TestNovelAICallToolImageAudience(t *testing.T) {
	srv := newNovelAIServer(t, &requestLog{})

	result := callTool(t, srv, "novelai", map[string]any{
		"model":  "nai-diffusion-5-full",
		"prompt": "a cat",
	})

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

	if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, []mcp.Role{present.RoleUser, present.RoleAssistant}) {
		t.Errorf("audience = %+v, want [user assistant]", img.Annotations)
	}
}

func TestNovelAILogsDoNotLeakSecrets(t *testing.T) {
	initImage := []byte("\x89PNG\r\n\x1a\n")
	initBase64 := base64.StdEncoding.EncodeToString(initImage)

	log, logs := observedLogger()

	h := &Clients{
		Log:      log,
		NovelAI:  newNovelAIBackend(t, &requestLog{}),
		Store:    newTestStore(t),
		Resolver: newResolver(t),
	}

	in := novelaiInput{
		generationInput: generationInput{
			Prompt:       "a cat",
			InitImageURL: newInitImageURL(t),
		},
		Model: "nai-diffusion-5-full",
	}

	if _, _, err := h.novelai(context.Background(), nil, in); err != nil {
		t.Fatalf("novelai() error: %v", err)
	}

	failing := &Clients{
		Log:     log,
		NovelAI: novelai.New("http://127.0.0.1:1", "sk-test"),
		Store:   newTestStore(t),
	}

	if _, _, err := failing.novelai(context.Background(), nil, novelaiInput{
		generationInput: generationInput{Prompt: "a cat"},
		Model:           "nai-diffusion-5-full",
	}); err == nil {
		t.Fatal("novelai() error = nil, want an error")
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
