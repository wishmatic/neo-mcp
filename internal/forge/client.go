package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wishmatic/neo-mcp/internal/utils"
	"go.uber.org/zap"
)

type Client struct {
	baseURL string
	http    *http.Client
	log     *zap.Logger
}

func New(baseURL string, log *zap.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
		log:     log,
	}
}

// postJSON sends a POST request directly to the Forge API.
//
// It is the responsibility of the calling client method to ensure that the payload is valid for
// the given endpoint, especially given that extensions may add more valid fields to the payload.
func (c *Client) postJSON[T any](ctx context.Context, label, path string, payload any) (T, error) {
	var out T

	body, err := json.Marshal(payload)
	if err != nil {
		return out, fmt.Errorf("marshal %s payload: %w", label, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("build %s request: %w", label, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("call %s: %w", label, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return out, utils.FmtHTTPErr(http.MethodPost, path, resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("decode %s response: %w", label, err)
	}

	return out, nil
}
