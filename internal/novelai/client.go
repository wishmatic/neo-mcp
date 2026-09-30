// Package novelai is NovelAI's image generation API in two layers: Generate is the domain operation, and the
// unexported requests, parameters, and streaming under it are the client that speaks NovelAI's own API.
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

const (
	DefaultBaseURL = "https://image.novelai.net"
	generatePath   = "/ai/generate-image-stream"
)

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	randomSeed func() uint32
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		http:       &http.Client{},
		randomSeed: rand.Uint32,
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
		return nil, utils.FmtHTTPErr(http.MethodPost, generatePath, resp)
	}

	image, err := decodeFinalImage(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	return [][]byte{image}, nil
}

func (c *Client) resolveSeed(requested int) uint32 {
	if requested < 0 {
		return c.randomSeed()
	}

	return uint32(requested)
}
