package novelai

import "testing"

func TestBaseParamsDefaults(t *testing.T) {
	params := marshalParams(t, baseParams(paramsInput{
		Prompt: "a cat",
		Steps:  23,
		Width:  832,
		Height: 1216,
		Scale:  5,
		Seed:   42,
	}))

	if got := asFloat(t, params, "params_version"); got != 3 {
		t.Errorf("params_version = %v, want 3", got)
	}

	if params["sampler"] != defaultSampler {
		t.Errorf("sampler = %v, want %s", params["sampler"], defaultSampler)
	}

	if params["negative_prompt"] != defaultNegativePrompt {
		t.Errorf("negative_prompt = %v, want the default negative prompt", params["negative_prompt"])
	}

	caption := nestedMap(t, nestedMap(t, params, "v4_prompt"), "caption")
	if caption["base_caption"] != "a cat" {
		t.Errorf("v4_prompt caption = %v, want a cat", caption["base_caption"])
	}

	negativeCaption := nestedMap(t, nestedMap(t, params, "v4_negative_prompt"), "caption")
	if negativeCaption["base_caption"] != defaultNegativePrompt {
		t.Errorf("v4_negative_prompt caption = %v, want the default negative prompt", negativeCaption["base_caption"])
	}

	if got := asFloat(t, params, "width"); got != 832 {
		t.Errorf("width = %v, want 832", got)
	}

	for _, key := range []string{"stream", "sm", "sm_dyn"} {
		if _, ok := params[key]; ok {
			t.Errorf("%s is present, want it absent", key)
		}
	}
}

func TestBaseParamsHonorsOverrides(t *testing.T) {
	params := marshalParams(t, baseParams(paramsInput{
		NegativePrompt: "bad",
		Sampler:        "k_dpmpp_2m",
	}))

	if params["sampler"] != "k_dpmpp_2m" {
		t.Errorf("sampler = %v, want k_dpmpp_2m", params["sampler"])
	}

	if params["negative_prompt"] != "bad" {
		t.Errorf("negative_prompt = %v, want bad", params["negative_prompt"])
	}

	negativeCaption := nestedMap(t, nestedMap(t, params, "v4_negative_prompt"), "caption")
	if negativeCaption["base_caption"] != "bad" {
		t.Errorf("v4_negative_prompt caption = %v, want bad", negativeCaption["base_caption"])
	}
}

func TestNormalizeDimensions(t *testing.T) {
	width, height, err := normalizeDimensions(833, 1217)
	if err != nil {
		t.Fatalf("normalizeDimensions() error: %v", err)
	}

	if width != 896 || height != 1280 {
		t.Errorf("dimensions = %dx%d, want 896x1280", width, height)
	}

	width, height, err = normalizeDimensions(64, 64)
	if err != nil {
		t.Fatalf("normalizeDimensions() error: %v", err)
	}

	if width != 64 || height != 64 {
		t.Errorf("dimensions = %dx%d, want 64x64", width, height)
	}
}

func TestNormalizeDimensionsRejectsOutOfRange(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {4096, 4096}, {-64, 512}} {
		if _, _, err := normalizeDimensions(size[0], size[1]); err == nil {
			t.Errorf("normalizeDimensions(%d, %d) error = nil, want an error", size[0], size[1])
		}
	}
}
