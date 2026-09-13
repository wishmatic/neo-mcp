package shortener

import (
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL     string
	apiKey      string
	expiryDelay int
	http        *http.Client
}

func New(apiURL, apiKey string, expiryDelay int) *Client {
	return &Client{
		baseURL:     strings.TrimRight(apiURL, "/"),
		apiKey:      apiKey,
		expiryDelay: expiryDelay,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}
