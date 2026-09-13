package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

type txt2imgInput struct {
	generationInput

	DenoisingStrength float64 `json:"denoising_strength,omitempty" jsonschema:"if HR is enabled, the denoising strength for the hi-res second pass"`
}

func registerTxt2Img(
	srv *mcp.Server,
	log *zap.Logger,
	client *sdwebui.Client,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "txt2img",
		Description: "Generate images synchronously via the local Stable Diffusion WebUI (Forge Neo) instance. Blocks until generation completes and returns the image(s).",
		InputSchema: txt2imgSchema(),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		in txt2imgInput,
	) (*mcp.CallToolResult, generationOutput, error) {
		log.Debug("tool called",
			zap.String("tool", "txt2img"),
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

		log.Info("txt2img generating synchronously",
			zap.String("model", in.Model),
			zap.Int("steps", in.SamplingSteps),
			zap.Int("width", in.Width),
			zap.Int("height", in.Height),
		)

		images, err := client.Txt2Img(ctx, sdwebui.Txt2ImgRequest{
			Checkpoint:             in.Model,
			ForgePreset:            in.ForgePreset,
			ForgeAdditionalModules: in.VAEAndTextModels,

			Prompt:         in.Prompt,
			NegativePrompt: in.NegativePrompt,
			Steps:          in.SamplingSteps,
			Width:          in.Width,
			Height:         in.Height,
			Seed:           in.Seed,
			CFGScale:       in.CFGScale,
			SamplerName:    in.SamplingMethod,
			Scheduler:      in.ScheduleType,

			EnableHR:          in.EnableHR,
			HRScale:           in.HRScale,
			HRUpscaler:        in.HRUpscaler,
			HRSecondPassSteps: in.HRSecondPassSteps,
			DenoisingStrength: in.DenoisingStrength,
			HRCFGScale:        in.HRCFGScale,
		})
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				log.Warn("txt2img aborted: request context cancelled before completion",
					zap.Error(err),
					zap.String("ctx_err", ctxErr.Error()),
				)
			} else {
				log.Error("txt2img generation failed", zap.Error(err))
			}

			return nil, generationOutput{}, fmt.Errorf("txt2img: %w", err)
		}

		log.Info("txt2img generation finished", zap.Int("images", len(images)))

		return publishImages(ctx, log, "txt2img", images, uploader, shortenerClient)
	})
}

func txt2imgSchema() *jsonschema.Schema {
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
	setDefault(s.Properties, "sampler_name", defaultSampler)
	setDefault(s.Properties, "scheduler", defaultScheduler)

	return s
}
