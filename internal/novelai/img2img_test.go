package novelai

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestImg2ImgRequest(t *testing.T) {
	server, captured := newGenerateServer(t, encodeFrame(t, map[string]any{
		"event_type": "final",
		"image":      []byte("out"),
	}))

	client := New(server.URL, "sk-test", false)
	client.randomSeed = func() uint32 { return 99 }

	images, err := client.Img2Img(context.Background(), Img2ImgRequest{
		Model:     "nai-diffusion-5-full",
		Prompt:    "a cat",
		Steps:     23,
		Width:     832,
		Height:    1216,
		Scale:     5,
		Seed:      42,
		InitImage: []byte("init"),
		Strength:  0.7,
	})
	if err != nil {
		t.Fatalf("Img2Img() error: %v", err)
	}

	if len(images) != 1 || string(images[0]) != "out" {
		t.Fatalf("images = %q, want one out", images)
	}

	body := decodeJSON(t, captured.body)

	if body["action"] != "img2img" {
		t.Errorf("action = %v, want img2img", body["action"])
	}

	params := nestedMap(t, body, "parameters")

	if params["image"] != base64.StdEncoding.EncodeToString([]byte("init")) {
		t.Errorf("image = %v, want the raw base64 init image", params["image"])
	}

	if image, ok := params["image"].(string); ok && strings.HasPrefix(image, "data:") {
		t.Errorf("image = %q, want no data URI prefix", image)
	}

	if got := asFloat(t, params, "strength"); got != 0.7 {
		t.Errorf("strength = %v, want 0.7", got)
	}

	if _, ok := params["noise"]; !ok {
		t.Error("noise is missing, want an explicit 0")
	}

	if got := asFloat(t, params, "noise"); got != 0 {
		t.Errorf("noise = %v, want 0", got)
	}

	if got := asFloat(t, params, "extra_noise_seed"); got != 99 {
		t.Errorf("extra_noise_seed = %v, want 99", got)
	}

	for _, key := range []string{"stream", "sm", "sm_dyn"} {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present, want it absent", key)
		}
	}
}
