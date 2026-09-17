package novelai

import "fmt"

const (
	defaultSampler = "k_euler_ancestral"

	dimensionMultiple = 64
	minPixels         = dimensionMultiple * dimensionMultiple
	maxPixels         = 3047424
)

const defaultNegativePrompt = "lowres, artistic error, film grain, scan artifacts, worst quality, bad quality, " +
	"jpeg artifacts, very displeasing, chromatic aberration, dithering, halftone, screentone, multiple views, " +
	"logo, too many watermarks, negative space, blank page, multiple views, character sheet, reference sheet, " +
	"turnaround, front and back, back view, from behind, multiple angles, split screen, dual view, text"

type paramsInput struct {
	Prompt         string
	NegativePrompt string
	Sampler        string
	Steps          int
	Width          int
	Height         int
	Scale          float64
	Seed           uint32
}

func baseParams(in paramsInput) map[string]any {
	sampler := in.Sampler
	if sampler == "" {
		sampler = defaultSampler
	}

	negativePrompt := in.NegativePrompt
	if negativePrompt == "" {
		negativePrompt = defaultNegativePrompt
	}

	return map[string]any{
		"params_version":                        3,
		"sampler":                               sampler,
		"seed":                                  in.Seed,
		"n_samples":                             1,
		"noise_schedule":                        "karras",
		"image_format":                          "png",
		"ucPreset":                              0,
		"qualityToggle":                         true,
		"autoSmea":                              false,
		"dynamic_thresholding":                  false,
		"controlnet_strength":                   1,
		"legacy":                                false,
		"add_original_image":                    true,
		"cfg_rescale":                           0,
		"legacy_v3_extend":                      false,
		"skip_cfg_above_sigma":                  nil,
		"use_coords":                            false,
		"legacy_uc":                             false,
		"normalize_reference_strength_multiple": true,
		"inpaintImg2ImgStrength":                1,
		"deliberate_euler_ancestral_bug":        false,
		"prefer_brownian":                       true,
		"characterPrompts":                      []any{},
		"width":                                 in.Width,
		"height":                                in.Height,
		"scale":                                 in.Scale,
		"steps":                                 in.Steps,
		"negative_prompt":                       negativePrompt,
		"v4_prompt": map[string]any{
			"caption": map[string]any{
				"base_caption":  in.Prompt,
				"char_captions": []any{},
			},
			"use_coords": false,
			"use_order":  true,
		},
		"v4_negative_prompt": map[string]any{
			"caption": map[string]any{
				"base_caption":  negativePrompt,
				"char_captions": []any{},
			},
			"legacy_uc": false,
		},
	}
}

func normalizeDimensions(width, height int) (int, int, error) {
	width = roundUpToMultiple(width)
	height = roundUpToMultiple(height)

	total := width * height
	if total < minPixels || total > maxPixels {
		return 0, 0, fmt.Errorf(
			"novelai: dimensions %dx%d (%d px) are outside the allowed %d to %d px range",
			width, height, total, minPixels, maxPixels,
		)
	}

	return width, height, nil
}

func roundUpToMultiple(value int) int {
	if value <= 0 {
		return 0
	}

	return ((value + dimensionMultiple - 1) / dimensionMultiple) * dimensionMultiple
}
