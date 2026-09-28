package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// imageGenerationAnnotations: the generation tools fetch remote inputs, spend credits and store new images, so they are
// not idempotent, but they only ever add resources rather than overwrite or delete existing ones.
func imageGenerationAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: new(false),
		IdempotentHint:  false,
		OpenWorldHint:   new(true),
	}
}
