package forge

import "strings"

var knownModelExtensions = []string{
	".safetensors",
	".gguf",
	".ckpt",
	".pt",
	".bin",
}

type overrideSettings struct {
	Checkpoint string   `json:"sd_model_checkpoint,omitempty"`
	Preset     string   `json:"forge_preset,omitempty"`
	Modules    []string `json:"forge_additional_modules,omitempty"`
}

// applyOverrideSettings adds override settings to the payload.
//
// Forge doesn't provide a direct way to provide checkpoint and additional modules, etc. so they've
// designed the API to accept a single override_settings object. Callers should be careful about
// what a call actually needs to override, as a call is potentially stateful; e.g., defaults
// missing overrides can use what was last set in the UI. This can make generations
// non-deterministic.
func applyOverrideSettings(payload map[string]any, checkpoint, preset string, modules []string) {
	if checkpoint == "" && preset == "" && len(modules) == 0 {
		return
	}

	normalized := make([]string, 0, len(modules))
	for _, m := range modules {
		normalized = append(normalized, ensureSafetensors(m))
	}

	payload["override_settings"] = overrideSettings{
		Checkpoint: ensureSafetensors(checkpoint),
		Preset:     preset,
		Modules:    normalized,
	}
}

func ensureSafetensors(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}

	lower := strings.ToLower(name)
	for _, ext := range knownModelExtensions {
		if strings.HasSuffix(lower, ext) {
			return name
		}
	}

	return name + ".safetensors"
}
