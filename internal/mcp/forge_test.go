package mcp

import (
	"encoding/base64"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newForgeServer(t *testing.T, log *requestLog) *mcp.Server {
	t.Helper()

	srv, err := New(Clients{
		Log:      zapNop(),
		Forge:    newForgeBackend(t, log),
		Store:    newTestStore(t),
		Resolver: newResolver(t),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return srv
}

func TestForgeCallToolUsesTxt2ImgWithoutAnInitImage(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":        "sd_xl_base_1.0",
		"forge_preset": "krea",
		"prompt":       "a cat",
	})

	if path := singleRequestPath(t, log); path != "/sdapi/v1/txt2img" {
		t.Errorf("path = %q, want /sdapi/v1/txt2img", path)
	}

	body := singleRequestBody(t, log)

	if body["prompt"] != "a cat" {
		t.Errorf("prompt = %v, want a cat", body["prompt"])
	}

	if body["sampler_name"] != forgeDefaultSampler {
		t.Errorf("sampler_name = %v, want %s", body["sampler_name"], forgeDefaultSampler)
	}

	if body["scheduler"] != forgeDefaultScheduler {
		t.Errorf("scheduler = %v, want %s", body["scheduler"], forgeDefaultScheduler)
	}

	if got := numberField(t, body, "width"); got != 512 {
		t.Errorf("width = %v, want the 512 the description documents", got)
	}

	if got := numberField(t, body, "steps"); got != 20 {
		t.Errorf("steps = %v, want the schema default 20", got)
	}

	if _, ok := body["init_images"]; ok {
		t.Error("init_images is present, want none without an init image")
	}

	overrides, ok := body["override_settings"].(map[string]any)
	if !ok {
		t.Fatalf("override_settings = %T, want the model overrides", body["override_settings"])
	}

	if overrides["sd_model_checkpoint"] != "sd_xl_base_1.0.safetensors" {
		t.Errorf("checkpoint override = %v, want sd_xl_base_1.0.safetensors", overrides["sd_model_checkpoint"])
	}

	if overrides["forge_preset"] != "krea" {
		t.Errorf("preset override = %v, want krea", overrides["forge_preset"])
	}
}

func TestForgeCallToolUsesImg2ImgWithAnInitImage(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":              "sd_xl_base_1.0.safetensors",
		"forge_preset":       "",
		"prompt":             "a cat",
		"init_image_url":     newSizedInitImageURL(t, 1024, 1024),
		"denoising_strength": 0.6,
	})

	if path := singleRequestPath(t, log); path != "/sdapi/v1/img2img" {
		t.Errorf("path = %q, want /sdapi/v1/img2img", path)
	}

	body := singleRequestBody(t, log)

	if got := numberField(t, body, "denoising_strength"); got != 0.6 {
		t.Errorf("denoising_strength = %v, want 0.6", got)
	}

	images, ok := body["init_images"].([]any)
	if !ok || len(images) != 1 {
		t.Fatalf("init_images = %v, want one image", body["init_images"])
	}

	encoded, ok := images[0].(string)
	if !ok || encoded == "" {
		t.Fatalf("init_images[0] = %v, want raw base64", images[0])
	}

	if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
		t.Errorf("init_images[0] decode error: %v", err)
	}

	if _, ok := body["enable_hr"]; ok {
		t.Error("enable_hr is present, want none when HR is off")
	}
}

func TestForgeCallToolTakesSizeFromInitImage(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":          "sd_xl_base_1.0.safetensors",
		"forge_preset":   "",
		"prompt":         "a cat",
		"init_image_url": newSizedInitImageURL(t, 1003, 667),
	})

	body := singleRequestBody(t, log)

	width, height := numberField(t, body, "width"), numberField(t, body, "height")
	if width != 1000 || height != 664 {
		t.Errorf("dimensions = %vx%v, want the init image's 1003x667 on the 8px grid", width, height)
	}
}

func TestForgeCallToolKeepsRequestedSize(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":          "sd_xl_base_1.0.safetensors",
		"forge_preset":   "",
		"prompt":         "a cat",
		"init_image_url": newSizedInitImageURL(t, 1024, 1024),
		"width":          832,
		"height":         1216,
	})

	body := singleRequestBody(t, log)

	width, height := numberField(t, body, "width"), numberField(t, body, "height")
	if width != 832 || height != 1216 {
		t.Errorf("dimensions = %vx%v, want the requested 832x1216", width, height)
	}
}

func TestForgeCallToolDefaultsImg2ImgDenoisingStrength(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":          "sd_xl_base_1.0.safetensors",
		"forge_preset":   "",
		"prompt":         "a cat",
		"init_image_url": newInitImageURL(t),
	})

	body := singleRequestBody(t, log)

	if got := numberField(t, body, "denoising_strength"); got != 0.75 {
		t.Errorf("denoising_strength = %v, want the 0.75 the description documents", got)
	}
}

func TestForgeCallToolPassesValuesThrough(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":           "sd_xl_base_1.0.safetensors",
		"forge_preset":    "",
		"prompt":          "a cat",
		"negative_prompt": "bad",
		"sampler_name":    "Euler",
		"scheduler":       "Karras",
		"steps":           30,
		"width":           768,
		"height":          512,
		"cfg_scale":       6.5,
		"seed":            42,
	})

	body := singleRequestBody(t, log)

	if body["negative_prompt"] != "bad" || body["sampler_name"] != "Euler" || body["scheduler"] != "Karras" {
		t.Errorf("body = %v, want the sampler and prompt fields copied", body)
	}

	for field, want := range map[string]float64{
		"steps":     30,
		"width":     768,
		"height":    512,
		"cfg_scale": 6.5,
		"seed":      42,
	} {
		if got := numberField(t, body, field); got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}
}

func TestForgeCallToolMapsHRFields(t *testing.T) {
	log := &requestLog{}
	srv := newForgeServer(t, log)

	callTool(t, srv, "forge", map[string]any{
		"model":                "sd_xl_base_1.0.safetensors",
		"forge_preset":         "",
		"prompt":               "a cat",
		"enable_hr":            true,
		"hr_scale":             2,
		"hr_upscaler":          "4x-AnimeSharp",
		"hr_second_pass_steps": 10,
		"hr_cfg":               5,
		"denoising_strength":   0.4,
		"vae_and_text_models":  []string{"vae.safetensors", "text_encoder"},
	})

	body := singleRequestBody(t, log)

	if body["enable_hr"] != true {
		t.Errorf("enable_hr = %v, want true", body["enable_hr"])
	}

	for field, want := range map[string]float64{
		"hr_scale":             2,
		"hr_second_pass_steps": 10,
		"hr_cfg":               5,
		"denoising_strength":   0.4,
	} {
		if got := numberField(t, body, field); got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}

	if body["hr_upscaler"] != "4x-AnimeSharp" {
		t.Errorf("hr_upscaler = %v, want 4x-AnimeSharp", body["hr_upscaler"])
	}

	if !slices.Equal(body["hr_additional_modules"].([]any), []any{"Use same choices"}) {
		t.Errorf("hr_additional_modules = %v, want Use same choices", body["hr_additional_modules"])
	}

	overrides := body["override_settings"].(map[string]any)

	if !slices.Equal(overrides["forge_additional_modules"].([]any), []any{"vae.safetensors", "text_encoder.safetensors"}) {
		t.Errorf("module overrides = %v, want them with the safetensors extension", overrides["forge_additional_modules"])
	}
}
