package sdwebui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Img2ImgRequest struct {
	Checkpoint             string   `json:"-"`
	ForgePreset            string   `json:"-"`
	ForgeAdditionalModules []string `json:"-"`

	InitImageData []byte `json:"-"`

	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`

	SamplerName string `json:"sampler_name"`
	Scheduler   string `json:"scheduler"`
	Steps       int    `json:"steps"`

	Width    int     `json:"width"`
	Height   int     `json:"height"`
	CFGScale float64 `json:"cfg_scale"`

	DenoisingStrength float64 `json:"denoising_strength"`

	Seed int `json:"seed"`

	EnableHR          bool    `json:"enable_hr"`
	HRScale           float64 `json:"hr_scale"`
	HRUpscaler        string  `json:"hr_upscaler"`
	HRSecondPassSteps int     `json:"hr_second_pass_steps"`
	HRCFGScale        float64 `json:"hr_cfg"`
}

func (c *Client) Img2Img(ctx context.Context, req Img2ImgRequest) ([][]byte, error) {
	payload := map[string]any{
		"init_images":        []string{base64DataURI(req.InitImageData)},
		"prompt":             req.Prompt,
		"negative_prompt":    req.NegativePrompt,
		"steps":              req.Steps,
		"width":              req.Width,
		"height":             req.Height,
		"seed":               req.Seed,
		"cfg_scale":          req.CFGScale,
		"sampler_name":       req.SamplerName,
		"scheduler":          req.Scheduler,
		"denoising_strength": req.DenoisingStrength,
	}

	applyOverrideSettings(payload, req.Checkpoint, req.ForgePreset, req.ForgeAdditionalModules)

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

		if req.HRCFGScale != 0 {
			payload["hr_cfg"] = req.HRCFGScale
		}

		payload["hr_additional_modules"] = []string{"Use same choices"}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal img2img payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/sdapi/v1/img2img", bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("build img2img request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call img2img: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", c.httpError(http.MethodPost, "/sdapi/v1/img2img", resp))
	}

	var out imagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode img2img response: %w", err)
	}

	return decodeImages(out)
}

func (c *Client) FetchImage(ctx context.Context, url string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build fetch request: %w", err)
	}

	resp, err := c.fetchHTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", c.httpError(http.MethodGet, url, resp))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image body: %w", err)
	}

	return data, nil
}

func base64DataURI(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}
