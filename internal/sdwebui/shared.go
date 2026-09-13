package sdwebui

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

type Client struct {
	baseURL       string
	http          *http.Client
	fetchHTTP     *http.Client
	verboseErrors bool
}

func New(baseURL string, verboseErrors bool) *Client {
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		http:          &http.Client{},
		fetchHTTP:     &http.Client{Timeout: 60 * time.Second},
		verboseErrors: verboseErrors,
	}
}

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

// httpError builds a human-readable error for a non-2xx HTTP response.
func (c *Client) httpError(method, path string, resp *http.Response) string {
	body := utils.ReadLimited(resp.Body)

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
