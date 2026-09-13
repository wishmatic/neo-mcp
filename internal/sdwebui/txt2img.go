package sdwebui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Txt2ImgRequest struct {
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

func (c *Client) Txt2Img(ctx context.Context, req Txt2ImgRequest) ([][]byte, error) {
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

	if req.EnableHR {
		payload["enable_hr"] = true

		if req.HRScale != 0 {
			payload["hr_scale"] = req.HRScale
		}

		if req.HRUpscaler != "" {
			payload["hr_upscaler"] = req.HRUpscaler
		}

		if req.HRSecondPassSteps != 0 {
			payload["hr_second_pass_steps"] = req.HRSecondPassSteps
		}

		if req.DenoisingStrength != 0 {
			payload["denoising_strength"] = req.DenoisingStrength
		}

		if req.HRCFGScale != 0 {
			payload["hr_cfg"] = req.HRCFGScale
		}

		payload["hr_additional_modules"] = []string{"Use same choices"}
	}

	applyOverrideSettings(payload, req.Checkpoint, req.ForgePreset, req.ForgeAdditionalModules)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal txt2img payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sdapi/v1/txt2img", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build txt2img request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call txt2img: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", c.httpError(http.MethodPost, "/sdapi/v1/txt2img", resp))
	}

	var out imagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode txt2img response: %w", err)
	}

	return decodeImages(out)
}
