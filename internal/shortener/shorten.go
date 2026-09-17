package shortener

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

type newRequest struct {
	LongLink    string `json:"longlink"`
	ExpiryDelay int    `json:"expiry_delay"`
}

type newResponse struct {
	IsSuccess  bool   `json:"success"`
	IsError    bool   `json:"error"`
	ShortURL   string `json:"shorturl"`
	ExpiryTime int64  `json:"expiry_time"`
	Reason     string `json:"reason"`
}

func (c *Client) Shorten(ctx context.Context, longURL string) (string, error) {
	body, err := json.Marshal(newRequest{LongLink: longURL, ExpiryDelay: c.expiryDelay})
	if err != nil {
		return "", fmt.Errorf("marshal shorten request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/new", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build shorten request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("call shortener: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg := utils.ReadLimited(resp.Body)

		return "", fmt.Errorf("shortener returned HTTP %d: %s", resp.StatusCode, msg)
	}

	var out newResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode shorten response: %w", err)
	}

	if !out.IsSuccess || out.IsError {
		reason := strings.TrimSpace(out.Reason)
		if reason == "" {
			reason = "unknown error"
		}

		return "", fmt.Errorf("shortener failed: %s", reason)
	}

	short := strings.TrimSpace(out.ShortURL)
	if short == "" {
		return "", fmt.Errorf("shortener returned an empty shorturl")
	}

	if !utils.IsHTTP(short) {
		short = resolveRelative(c.baseURL, short)
	}

	return short, nil
}

func resolveRelative(baseURL, short string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return short
	}

	ref, err := url.Parse(short)
	if err != nil {
		return short
	}

	return u.ResolveReference(ref).String()
}
