package sdwebui

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type overrideSettings struct {
	Checkpoint             string   `json:"sd_model_checkpoint,omitempty"`
	ForgePreset            string   `json:"forge_preset,omitempty"`
	ForgeAdditionalModules []string `json:"forge_additional_modules,omitempty"`
}

type imagesResponse struct {
	Images []string `json:"images"`
}

func applyOverrideSettings(payload map[string]any, checkpoint, preset string, modules []string) {
	if checkpoint == "" && preset == "" && len(modules) == 0 {
		return
	}

	normalized := make([]string, 0, len(modules))
	for _, m := range modules {
		normalized = append(normalized, ensureSafetensors(m))
	}

	payload["override_settings"] = overrideSettings{
		Checkpoint:             ensureSafetensors(checkpoint),
		ForgePreset:            preset,
		ForgeAdditionalModules: normalized,
	}
}

func decodeImages(out imagesResponse) ([][]byte, error) {
	images := make([][]byte, 0, len(out.Images))
	for _, b64 := range out.Images {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("decode image data: %w", err)
		}

		images = append(images, data)
	}

	return images, nil
}

var knownModelExtensions = []string{
	".safetensors",
	".gguf",
	".ckpt",
	".pt",
	".bin",
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
