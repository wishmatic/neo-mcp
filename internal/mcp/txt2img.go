package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

type txt2imgInput struct {
	generationInput

	DenoisingStrength float64 `json:"denoising_strength,omitempty" jsonschema:"if HR is enabled, the denoising strength for the hi-res second pass"`
}

func registerTxt2Img(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "txt2img",
		Description: "Generate images synchronously via the local Stable Diffusion WebUI (Forge Neo) instance, " +
			"or via NovelAI when the model is a NovelAI model id. Blocks until generation completes and returns the image(s).",
		InputSchema: txt2imgSchema(h.defaultFormat),
	}, h.txt2img)
}

func (h *handlers) txt2img(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in txt2imgInput,
) (*mcp.CallToolResult, generationOutput, error) {
	provider := imagegen.ProviderOf(in.Model)

	format, err := h.outputFormat(in.Format)
	if err != nil {
		return nil, generationOutput{}, fmt.Errorf("txt2img: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "txt2img"),
		zap.String("provider", provider),
		zap.String("format", format.String()),
		zap.String("model", in.Model),
		zap.String("forge_preset", in.ForgePreset),
		zap.Strings("vae_and_text_models", in.VAEAndTextModels),
		zap.String("sampler", in.SamplingMethod),
		zap.String("scheduler", in.ScheduleType),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
		zap.Float64("cfg_scale", in.CFGScale),
		zap.Int("seed", in.Seed),
		zap.Bool("enable_hr", in.EnableHR),
		zap.Float64("hr_scale", in.HRScale),
		zap.String("hr_upscaler", in.HRUpscaler),
		zap.Int("hr_second_pass_steps", in.HRSecondPassSteps),
		zap.Float64("denoising_strength", in.DenoisingStrength),
		zap.Float64("hr_cfg", in.HRCFGScale),
	)

	h.log.Info("txt2img generating synchronously",
		zap.String("provider", provider),
		zap.String("model", in.Model),
		zap.Int("steps", in.SamplingSteps),
		zap.Int("width", in.Width),
		zap.Int("height", in.Height),
	)

	images, err := h.gen.Txt2Img(ctx, imagegenTxt2ImgRequest(in))
	if err != nil {
		return nil, generationOutput{}, h.generationFailure(ctx, "txt2img", err)
	}

	h.log.Info("txt2img generation finished", zap.Int("images", len(images)))

	result, out, err := h.publishImages(ctx, "txt2img", images, format)
	if err != nil {
		return nil, generationOutput{}, err
	}

	return result, out, nil
}

func txt2imgSchema(def imgfmt.Format) *jsonschema.Schema {
	s, err := jsonschema.For[txt2imgInput](nil)
	if err != nil {
		panic(fmt.Sprintf("txt2img: infer input schema: %v", err))
	}

	setDefault(s.Properties, "negative_prompt", "")
	setDefault(s.Properties, "steps", 20)
	setDefault(s.Properties, "width", 512)
	setDefault(s.Properties, "height", 512)
	setDefault(s.Properties, "seed", -1)
	setDefault(s.Properties, "cfg_scale", 7.0)
	setDefault(s.Properties, "sampler_name", "")
	setDefault(s.Properties, "scheduler", "")
	setFormatSchema(s, def)

	return s
}
