package openai

import (
	_ "embed"
	"strings"
)

// DefaultSystemPrompt is the system instruction used when the configured default supplies none.
//
//go:embed system_prompt.txt
var DefaultSystemPrompt string

const DefaultPrompt = "Describe this image in detail."

func (c *Client) systemPrompt() string {
	if configured := strings.TrimSpace(c.configuredSystemPrompt); configured != "" {
		return configured
	}

	return DefaultSystemPrompt
}

func (c *Client) prompt() string {
	if configured := strings.TrimSpace(c.configuredPrompt); configured != "" {
		return configured
	}

	return DefaultPrompt
}
