package forge

import (
	"slices"
	"testing"
)

func TestApplyUpscaleSettingsDisabled(t *testing.T) {
	payload := map[string]any{"prompt": "test"}

	applyUpscaleSettings(payload, upscaleSettings{Scale: 2, Upscaler: "4x", SecondPassSteps: 10})

	if len(payload) != 1 {
		t.Errorf("payload = %v, want the hi-res pass to be left out", payload)
	}
}

func TestApplyUpscaleSettingsAllFields(t *testing.T) {
	payload := map[string]any{}

	applyUpscaleSettings(payload, upscaleSettings{
		IsEnabled:         true,
		Scale:             2,
		Upscaler:          "4x-AnimeSharp",
		SecondPassSteps:   12,
		CFGScale:          5,
		DenoisingStrength: 0.4,
	})

	want := map[string]any{
		"enable_hr":             true,
		"hr_scale":              2.0,
		"hr_upscaler":           "4x-AnimeSharp",
		"hr_second_pass_steps":  12,
		"hr_cfg":                5.0,
		"denoising_strength":    0.4,
		"hr_additional_modules": []string{"Use same choices"},
	}

	for key, wantValue := range want {
		if got := payload[key]; !equalValue(got, wantValue) {
			t.Errorf("%s = %v, want %v", key, got, wantValue)
		}
	}

	if len(payload) != len(want) {
		t.Errorf("payload = %v, want exactly %d keys", payload, len(want))
	}
}

func TestApplyUpscaleSettingsOnlyEnabled(t *testing.T) {
	payload := map[string]any{}

	applyUpscaleSettings(payload, upscaleSettings{IsEnabled: true})

	if payload["enable_hr"] != true {
		t.Errorf("enable_hr = %v, want true", payload["enable_hr"])
	}

	if len(payload) != 2 {
		t.Errorf("payload = %v, want only the pass switch and the shared modules", payload)
	}
}

func equalValue(got, want any) bool {
	gotSlice, gotIsSlice := got.([]string)
	wantSlice, wantIsSlice := want.([]string)

	if gotIsSlice || wantIsSlice {
		return gotIsSlice && wantIsSlice && slices.Equal(gotSlice, wantSlice)
	}

	return got == want
}
