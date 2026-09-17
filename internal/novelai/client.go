package novelai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

const generatePath = "/ai/generate-image-stream"

type Client struct {
	baseURL         string
	apiKey          string
	http            *http.Client
	isVerboseErrors bool
	randomSeed      func() uint32
}

func New(baseURL, apiKey string, isVerboseErrors bool) *Client {
	return &Client{
		baseURL:         strings.TrimRight(baseURL, "/"),
		apiKey:          apiKey,
		http:            &http.Client{},
		isVerboseErrors: isVerboseErrors,
		randomSeed:      rand.Uint32,
	}
}

func (c *Client) generate(ctx context.Context, label string, payload map[string]any) ([][]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", label, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+generatePath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", label, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", label, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", c.httpError(http.MethodPost, generatePath, resp))
	}

	image, err := decodeFinalImage(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	return [][]byte{image}, nil
}

func (c *Client) get(ctx context.Context, label, path string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", label, err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call %s: %w", label, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", c.httpError(http.MethodGet, path, resp))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", label, err)
	}

	return nil
}

func (c *Client) resolveSeed(requested int) uint32 {
	if requested < 0 {
		return c.randomSeed()
	}

	return uint32(requested)
}

// httpError builds a human-readable error for a non-2xx HTTP response.
func (c *Client) httpError(method, path string, resp *http.Response) string {
	body := utils.ReadLimited(resp.Body)

	if c.isVerboseErrors {
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
