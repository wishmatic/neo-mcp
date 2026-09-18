package openai

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	requestTimeout = 3 * time.Minute

	DefaultMaxTokens = 8192
)

type Config struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	Prompt       string
	MaxTokens    int
}

type Client struct {
	baseURL                string
	apiKey                 string
	defaultModel           string
	configuredSystemPrompt string
	configuredPrompt       string
	maxTokens              int
	http                   *http.Client
}

func New(cfg Config) (*Client, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")

	u, err := url.Parse(trimmed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("openai: invalid base URL %q", cfg.BaseURL)
	}

	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	return &Client{
		baseURL:                trimmed,
		apiKey:                 cfg.APIKey,
		defaultModel:           cfg.Model,
		configuredSystemPrompt: cfg.SystemPrompt,
		configuredPrompt:       cfg.Prompt,
		maxTokens:              maxTokens,
		http:                   &http.Client{Timeout: requestTimeout},
	}, nil
}

func (c *Client) MaxTokens() int {
	return c.maxTokens
}
