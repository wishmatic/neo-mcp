package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
)

type generationInput struct {
	Model       string `json:"model" jsonschema:"checkpoint (model) filename to load, or a NovelAI model id starting with nai-diffusion- for the NovelAI backend; the id is passed through as-is"`
	ForgePreset string `json:"forge_preset,omitempty" jsonschema:"Forge UI preset; ignored by NovelAI"`

	VAEAndTextModels []string `json:"vae_and_text_models,omitempty" jsonschema:"VAE and text encoder model filenames to load. Leave empty to use the defaults bundled with the checkpoint; ignored by NovelAI"`

	Prompt         string `json:"prompt" jsonschema:"the text prompt describing the image to generate"`
	NegativePrompt string `json:"negative_prompt,omitempty" jsonschema:"things to avoid in the generated image"`

	NSFW bool `json:"nsfw,omitempty" jsonschema:"set this true when the generation is NSFW; it tags the saved example and places the image under an nsfw subdirectory"`

	SamplingMethod string `json:"sampler_name,omitempty" jsonschema:"the sampler to use; defaults to the backend's own sampler"`
	ScheduleType   string `json:"scheduler,omitempty" jsonschema:"the scheduler to use; ignored by NovelAI"`
	SamplingSteps  int    `json:"steps,omitempty" jsonschema:"number of sampling steps"`

	Width    int     `json:"width,omitempty" jsonschema:"output width in pixels"`
	Height   int     `json:"height,omitempty" jsonschema:"output height in pixels"`
	CFGScale float64 `json:"cfg_scale,omitempty" jsonschema:"classifier-free guidance scale"`

	Seed int `json:"seed,omitempty" jsonschema:"random seed; use -1 for a random seed"`

	EnableHR          bool    `json:"enable_hr,omitempty" jsonschema:"enable hi-res (HR) (second-pass) upscaling; ignored by NovelAI"`
	HRScale           float64 `json:"hr_scale,omitempty" jsonschema:"if HR is enabled, the hi-res upscaling factor (e.g. 2 for 2x); ignored by NovelAI"`
	HRUpscaler        string  `json:"hr_upscaler,omitempty" jsonschema:"if HR is enabled, the hi-res upscaler to use. Leave empty to disable upscaling; ignored by NovelAI"`
	HRSecondPassSteps int     `json:"hr_second_pass_steps,omitempty" jsonschema:"if HR is enabled, the number of steps for the hi-res second pass; ignored by NovelAI"`
	HRCFGScale        float64 `json:"hr_cfg,omitempty" jsonschema:"if HR is enabled, the CFG scale for the hi-res second pass; ignored by NovelAI"`

	formatInput
}

type formatInput struct {
	Format string `json:"format,omitempty" jsonschema:"output image format: png, jpeg, jxl (JPEG XL), or webp; defaults to the server's configured output format"`
}

// generationOutput is the structured output shared by the image tools. The generated images always travel in the call
// result's content as URLs, and URLs mirrors them for structured clients.
type generationOutput struct {
	Count int      `json:"count" jsonschema:"number of images generated"`
	URLs  []string `json:"urls" jsonschema:"URLs for the generated images"`
}

func setDefault(props map[string]*jsonschema.Schema, name string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal default for %s: %v", name, err))
	}

	props[name].Default = raw
}

func setFormatSchema(s *jsonschema.Schema, def imgfmt.Format) {
	setDefault(s.Properties, "format", def.String())

	names := make([]any, 0, len(imgfmt.Names()))
	for _, name := range imgfmt.Names() {
		names = append(names, name)
	}

	s.Properties["format"].Enum = names
}
