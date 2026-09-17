package openai

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const requestTimeout = 3 * time.Minute

type Client struct {
	baseURL             string
	apiKey              string
	defaultModel        string
	defaultSystemPrompt string
	http                *http.Client
}

func New(baseURL, apiKey, defaultModel, defaultSystemPrompt string) (*Client, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")

	u, err := url.Parse(trimmed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("openai: invalid base URL %q", baseURL)
	}

	return &Client{
		baseURL:             trimmed,
		apiKey:              apiKey,
		defaultModel:        defaultModel,
		defaultSystemPrompt: defaultSystemPrompt,
		http:                &http.Client{Timeout: requestTimeout},
	}, nil
}
