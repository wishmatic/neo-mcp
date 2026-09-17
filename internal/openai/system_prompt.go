package openai

import (
	_ "embed"
	"strings"
)

// DefaultSystemPrompt is the system instruction used when neither the call nor the configured default supplies one.
//
//go:embed system_prompt.txt
var DefaultSystemPrompt string

func (c *Client) systemPrompt() string {
	if configured := strings.TrimSpace(c.defaultSystemPrompt); configured != "" {
		return configured
	}

	return DefaultSystemPrompt
}
