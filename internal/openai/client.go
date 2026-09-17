package openai

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL      string
	apiKey       string
	defaultModel string
	http         *http.Client
}

func New(baseURL, apiKey, defaultModel string, timeout time.Duration) (*Client, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")

	u, err := url.Parse(trimmed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("openai: invalid base URL %q", baseURL)
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("openai: timeout must be positive, got %s", timeout)
	}

	return &Client{
		baseURL:      trimmed,
		apiKey:       apiKey,
		defaultModel: defaultModel,
		http:         &http.Client{Timeout: timeout},
	}, nil
}
