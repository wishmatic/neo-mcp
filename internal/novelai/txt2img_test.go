package novelai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTxt2ImgRequest(t *testing.T) {
	server, captured := newGenerateServer(t, encodeFrame(t, map[string]any{
		"event_type": "final",
		"image":      []byte("png"),
	}))

	client := New(server.URL, "sk-test", false)

	images, err := client.Txt2Img(context.Background(), Txt2ImgRequest{
		Model:  "nai-diffusion-5-full",
		Prompt: "a cat",
		Steps:  23,
		Width:  832,
		Height: 1216,
		Scale:  5,
		Seed:   42,
	})
	if err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	if len(images) != 1 || string(images[0]) != "png" {
		t.Fatalf("images = %q, want one png", images)
	}

	if captured.path != "/ai/generate-image-stream" {
		t.Errorf("path = %q, want /ai/generate-image-stream", captured.path)
	}

	if captured.authorization != "Bearer sk-test" {
		t.Errorf("authorization = %q, want Bearer sk-test", captured.authorization)
	}

	if captured.contentType != "application/json" {
		t.Errorf("content type = %q, want application/json", captured.contentType)
	}

	body := decodeJSON(t, captured.body)

	if body["action"] != "generate" {
		t.Errorf("action = %v, want generate", body["action"])
	}

	if body["model"] != "nai-diffusion-5-full" {
		t.Errorf("model = %v, want nai-diffusion-5-full", body["model"])
	}

	if body["input"] != "a cat" {
		t.Errorf("input = %v, want a cat", body["input"])
	}

	params := nestedMap(t, body, "parameters")

	if params["stream"] != "msgpack" {
		t.Errorf("stream = %v, want msgpack", params["stream"])
	}

	if got := asFloat(t, params, "width"); got != 832 {
		t.Errorf("width = %v, want 832", got)
	}

	if got := asFloat(t, params, "height"); got != 1216 {
		t.Errorf("height = %v, want 1216", got)
	}

	if got := asFloat(t, params, "steps"); got != 23 {
		t.Errorf("steps = %v, want 23", got)
	}

	if got := asFloat(t, params, "scale"); got != 5 {
		t.Errorf("scale = %v, want 5", got)
	}

	if got := asFloat(t, params, "seed"); got != 42 {
		t.Errorf("seed = %v, want 42", got)
	}

	caption := nestedMap(t, nestedMap(t, params, "v4_prompt"), "caption")
	if caption["base_caption"] != "a cat" {
		t.Errorf("v4_prompt caption = %v, want a cat", caption["base_caption"])
	}
}

func TestTxt2ImgRoundsDimensions(t *testing.T) {
	server, captured := newGenerateServer(t, encodeFrame(t, map[string]any{
		"event_type": "final",
		"image":      []byte("png"),
	}))

	client := New(server.URL, "sk-test", false)

	if _, err := client.Txt2Img(context.Background(), Txt2ImgRequest{
		Model:  "nai-diffusion-5-full",
		Width:  833,
		Height: 1217,
	}); err != nil {
		t.Fatalf("Txt2Img() error: %v", err)
	}

	params := nestedMap(t, decodeJSON(t, captured.body), "parameters")

	if got := asFloat(t, params, "width"); got != 896 {
		t.Errorf("width = %v, want 896", got)
	}

	if got := asFloat(t, params, "height"); got != 1280 {
		t.Errorf("height = %v, want 1280", got)
	}
}

func TestTxt2ImgRejectsBadDimensions(t *testing.T) {
	client := New("http://example.com", "sk-test", false)

	if _, err := client.Txt2Img(context.Background(), Txt2ImgRequest{
		Model:  "nai-diffusion-5-full",
		Width:  4096,
		Height: 4096,
	}); err == nil {
		t.Fatal("Txt2Img() error = nil, want an error")
	}
}

func TestTxt2ImgHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))

	t.Cleanup(server.Close)

	for _, verbose := range []bool{false, true} {
		client := New(server.URL, "sk-test", verbose)

		_, err := client.Txt2Img(context.Background(), Txt2ImgRequest{
			Model:  "nai-diffusion-5-full",
			Width:  512,
			Height: 512,
		})
		if err == nil {
			t.Fatalf("verbose %v: Txt2Img() error = nil, want an error", verbose)
		}

		if !strings.Contains(err.Error(), "500") {
			t.Errorf("verbose %v: error = %q, want the status", verbose, err.Error())
		}

		if !strings.Contains(err.Error(), "boom") {
			t.Errorf("verbose %v: error = %q, want the body", verbose, err.Error())
		}
	}
}

func TestTxt2ImgContextCancelled(t *testing.T) {
	server, _ := newGenerateServer(t, encodeFrame(t, map[string]any{
		"event_type": "final",
		"image":      []byte("png"),
	}))

	client := New(server.URL, "sk-test", false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Txt2Img(ctx, Txt2ImgRequest{
		Model:  "nai-diffusion-5-full",
		Width:  512,
		Height: 512,
	}); err == nil {
		t.Fatal("Txt2Img() error = nil, want an error")
	}
}
