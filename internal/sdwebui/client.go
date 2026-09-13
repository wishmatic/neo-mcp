package sdwebui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

type Client struct {
	baseURL       string
	http          *http.Client
	verboseErrors bool
}

func New(baseURL string, verboseErrors bool) *Client {
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		http:          &http.Client{},
		verboseErrors: verboseErrors,
	}
}

func postJSON[T any](ctx context.Context, c *Client, label, path string, payload any) (T, error) {
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
		return out, fmt.Errorf("%s", c.httpError(http.MethodPost, path, resp))
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("decode %s response: %w", label, err)
	}

	return out, nil
}

func (c *Client) postImages(ctx context.Context, label, path string, payload any) ([][]byte, error) {
	out, err := postJSON[imagesResponse](ctx, c, label, path, payload)
	if err != nil {
		return nil, err
	}

	return decodeImages(out)
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
