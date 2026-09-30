package forge

import (
	"context"
	"fmt"

	"github.com/wishmatic/neo-mcp/internal/format"
	"go.uber.org/zap"
)

type txt2imgRequest struct {
	Checkpoint             string   `json:"-"`
	ForgePreset            string   `json:"-"`
	ForgeAdditionalModules []string `json:"-"`

	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`

	SamplerName string `json:"sampler_name"`
	Scheduler   string `json:"scheduler"`
	Steps       int    `json:"steps"`

	Width    int     `json:"width"`
	Height   int     `json:"height"`
	CFGScale float64 `json:"cfg_scale"`

	Seed int `json:"seed"`

	EnableHR          bool    `json:"enable_hr"`
	HRScale           float64 `json:"hr_scale"`
	HRUpscaler        string  `json:"hr_upscaler"`
	HRSecondPassSteps int     `json:"hr_second_pass_steps"`
	DenoisingStrength float64 `json:"denoising_strength"`
	HRCFGScale        float64 `json:"hr_cfg"`
}

func (c *Client) txt2img(ctx context.Context, req txt2imgRequest) ([][]byte, error) {
	payload := map[string]any{
		"prompt":          req.Prompt,
		"negative_prompt": req.NegativePrompt,
		"steps":           req.Steps,
		"width":           req.Width,
		"height":          req.Height,
		"seed":            req.Seed,
		"cfg_scale":       req.CFGScale,
		"sampler_name":    req.SamplerName,
		"scheduler":       req.Scheduler,
	}

	applyOverrideSettings(payload, req.Checkpoint, req.ForgePreset, req.ForgeAdditionalModules)
	applyUpscaleSettings(payload, upscaleSettings{
		IsEnabled:         req.EnableHR,
		Scale:             req.HRScale,
		Upscaler:          req.HRUpscaler,
		SecondPassSteps:   req.HRSecondPassSteps,
		CFGScale:          req.HRCFGScale,
		DenoisingStrength: req.DenoisingStrength,
	})

	out, err := c.postJSON[imagesResponse](ctx, "txt2img", "/sdapi/v1/txt2img", payload)
	if err != nil {
		return nil, err
	}

	if len(out.Images) == 0 {
		return nil, fmt.Errorf("txt2img: response contained no images")
	} else if len(out.Images) > 1 {
		c.log.Warn("txt2img call returned more than one image, using the first",
			zap.String("endpoint", "txt2img"),
			zap.Int("images", len(out.Images)),
		)
	}

	image, err := format.DecodeRawBase64(out.Images[0])
	if err != nil {
		return nil, err
	}

	return [][]byte{image}, nil
}
