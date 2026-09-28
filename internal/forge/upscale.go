package forge

type upscaleSettings struct {
	IsEnabled bool

	Scale             float64
	Upscaler          string
	SecondPassSteps   int
	CFGScale          float64
	DenoisingStrength float64
}

func applyUpscaleSettings(payload map[string]any, s upscaleSettings) {
	if !s.IsEnabled {
		return
	}

	payload["enable_hr"] = true

	if s.Scale != 0 {
		payload["hr_scale"] = s.Scale
	}

	if s.Upscaler != "" {
		payload["hr_upscaler"] = s.Upscaler
	}

	if s.SecondPassSteps != 0 {
		payload["hr_second_pass_steps"] = s.SecondPassSteps
	}

	if s.CFGScale != 0 {
		payload["hr_cfg"] = s.CFGScale
	}

	if s.DenoisingStrength != 0 {
		payload["denoising_strength"] = s.DenoisingStrength
	}

	payload["hr_additional_modules"] = []string{"Use same choices"}
}
