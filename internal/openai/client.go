// Package openai is the OpenAI-compatible image generation API in two layers: GenerateImages is the domain operation,
// and the unexported request and response shapes under it are the client. Any number of endpoints can be configured,
// and each call names the one it wants.
package openai

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
	providers []Provider
	http      *http.Client
}

// New builds a client over providers, which ParseProviders returns. The first is the one a call that names none uses.
func New(providers []Provider) *Client {
	return &Client{providers: providers, http: &http.Client{}}
}

// ProviderNames lists the configured providers in configuration order.
func (c *Client) ProviderNames() []string {
	names := make([]string, 0, len(c.providers))

	for _, provider := range c.providers {
		names = append(names, provider.Name)
	}

	return names
}

// provider resolves the name a caller gave into a configured provider, which is the first one when it named none.
func (c *Client) provider(name string) (Provider, error) {
	if len(c.providers) == 0 {
		return Provider{}, fmt.Errorf("openai: no providers are configured")
	}

	if name == "" {
		return c.providers[0], nil
	}

	for _, provider := range c.providers {
		if strings.EqualFold(provider.Name, name) {
			return provider, nil
		}
	}

	return Provider{}, fmt.Errorf("openai: no provider named %q; configured: %s",
		name, strings.Join(c.ProviderNames(), ", "))
}

func (c *Client) postJSON(ctx context.Context, provider Provider, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}

	req.Header.Set("Content-Type", "application/json")

	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", path, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return utils.FmtHTTPErr(http.MethodPost, path, resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}

	return nil
}
