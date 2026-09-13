package sdwebui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func (c *Client) postImages(ctx context.Context, label, path string, payload any) ([][]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", label, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", label, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", label, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", c.httpError(http.MethodPost, path, resp))
	}

	var out imagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", label, err)
	}

	return decodeImages(out)
}

func (c *Client) fetch(ctx context.Context, url string) ([]byte, error) {
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
