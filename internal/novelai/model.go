package novelai

import "strings"

const modelPrefix = "nai-diffusion-"

// IsModel reports whether model names a NovelAI model. The prefix is the only routing signal; ids are passed through
// unvalidated so models the server has never heard of keep working.
func IsModel(model string) bool {
	return strings.HasPrefix(strings.TrimSpace(model), modelPrefix)
}
