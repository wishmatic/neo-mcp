package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"go.uber.org/zap"
)

// generationInput is the input the generation tools share, so its fields are described here.
type generationInput struct {
	Prompt         string `json:"prompt" jsonschema:"the text prompt describing the image to generate"`
	NegativePrompt string `json:"negative_prompt,omitempty" jsonschema:"things to avoid in the generated image"`

	SamplingMethod string `json:"sampler_name,omitempty"`
	SamplingSteps  int    `json:"steps,omitempty" jsonschema:"number of sampling steps"`

	// Width and Height have no schema default: the SDK applies those before the handler runs, which would hide a
	// caller that left them unset and so wants the init image's size.
	Width    int     `json:"width,omitempty"`
	Height   int     `json:"height,omitempty"`
	CFGScale float64 `json:"cfg_scale,omitempty" jsonschema:"classifier-free guidance scale"`

	Seed int `json:"seed,omitempty" jsonschema:"random seed; use -1 for a random seed"`

	// InitImageURL is what makes a call img2img; without it the call is txt2img.
	InitImageURL string `json:"init_image_url,omitempty"`

	// DenoisingStrength is 0 when the caller left it out, which the backend then reads as its own default on the
	// img2img path and as "use my own setting" on the Forge hi-res path.
	DenoisingStrength float64 `json:"denoising_strength,omitempty"`

	formatInput
}

func setGenerationDefaults(s *jsonschema.Schema) {
	setDefault(s.Properties, "negative_prompt", "")
	setDefault(s.Properties, "steps", 20)
	setDefault(s.Properties, "seed", -1)
	setDefault(s.Properties, "cfg_scale", 7.0)
}

func (c *Clients) initImage(ctx context.Context, tool, url string) ([]byte, error) {
	if url == "" {
		return nil, nil
	}

	image, err := c.Resolver.Fetch(ctx, url)
	if err != nil {
		c.Log.Error(tool+" failed to fetch the init image",
			zap.String("init_image_url", url),
			zap.Error(err),
		)

		return nil, fmt.Errorf("%s: fetch init image: %w", tool, err)
	}

	return image, nil
}
