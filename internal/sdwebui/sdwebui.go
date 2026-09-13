package sdwebui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maximumErrorBodySize = 4 * 1024 // 4 KB

type Client struct {
	baseURL       string
	http          *http.Client
	fetchHTTP     *http.Client
	verboseErrors bool
}

func New(baseURL string, verboseErrors bool) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),

		// Image generation can take minutes, so we leave the client timeout unset.

		http: &http.Client{},

		// Used to download init images for img2img; follows redirects by default.

		fetchHTTP:     &http.Client{Timeout: 60 * time.Second},
		verboseErrors: verboseErrors,
	}
}

type Txt2ImgRequest struct {
	// These three fields are serialized into the request's override_settings.

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

type overrideSettings struct {
	Checkpoint             string   `json:"sd_model_checkpoint,omitempty"`
	ForgePreset            string   `json:"forge_preset,omitempty"`
	ForgeAdditionalModules []string `json:"forge_additional_modules,omitempty"`
}

type txt2imgResponse struct {
	Images []string `json:"images"`
}

type Img2ImgRequest struct {
	// These three fields are serialized into the request's override_settings.

	Checkpoint             string   `json:"-"`
	ForgePreset            string   `json:"-"`
	ForgeAdditionalModules []string `json:"-"`

	// InitImageData is the raw image bytes to use as the starting image. It is serialized into init_images as a
	// base64 data URI.

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

// httpError builds a human-readable error for a non-2xx HTTP response.
func (c *Client) httpError(method, path string, resp *http.Response) string {
	body := readBodyLimited(resp)

	if c.verboseErrors {
		return fmt.Sprintf(
			"%s %s returned HTTP %d (%s): %s",
			method, path, resp.StatusCode, resp.Status, body,
		)
	}

	if body != "" {
		return fmt.Sprintf("%s %s returned HTTP %d: %s", method, path, resp.StatusCode, body)
	}

	return fmt.Sprintf("%s %s returned HTTP %d", method, path, resp.StatusCode)
}

func readBodyLimited(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, maximumErrorBodySize))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(b))
}

var knownModelExtensions = []string{
	".safetensors",
	".ckpt",
	".pt",
	".bin",
	".gguf",
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

	if req.Checkpoint != "" || req.ForgePreset != "" || len(req.ForgeAdditionalModules) > 0 {
		modules := make([]string, 0, len(req.ForgeAdditionalModules))
		for _, m := range req.ForgeAdditionalModules {
			modules = append(modules, ensureSafetensors(m))
		}

		payload["override_settings"] = overrideSettings{
			Checkpoint:             ensureSafetensors(req.Checkpoint),
			ForgePreset:            req.ForgePreset,
			ForgeAdditionalModules: modules,
		}
	}

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

	var out txt2imgResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode txt2img response: %w", err)
	}

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

	if req.Checkpoint != "" || req.ForgePreset != "" || len(req.ForgeAdditionalModules) > 0 {
		modules := make([]string, 0, len(req.ForgeAdditionalModules))
		for _, m := range req.ForgeAdditionalModules {
			modules = append(modules, ensureSafetensors(m))
		}

		payload["override_settings"] = overrideSettings{
			Checkpoint:             ensureSafetensors(req.Checkpoint),
			ForgePreset:            req.ForgePreset,
			ForgeAdditionalModules: modules,
		}
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

	var out txt2imgResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode img2img response: %w", err)
	}

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
