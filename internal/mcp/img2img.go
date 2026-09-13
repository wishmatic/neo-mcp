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

type img2imgInput struct {
	generationInput

	InitImageURL string `json:"init_image_url" jsonschema:"URL of the image to transform; the service downloads it (following redirects)"`

	DenoisingStrength float64 `json:"denoising_strength,omitempty" jsonschema:"how much to change the input image (0 keeps it identical, 1 ignores it)"`
}

func registerImg2Img(
	srv *mcp.Server,
	log *zap.Logger,
	client *sdwebui.Client,
	uploader *s3upload.Client,
	shortenerClient *shortener.Client,
) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "img2img",
		Description: "Transform an existing image. Downloads the input image from a URL (following redirects), then blocks until generation completes and returns the image.",
		InputSchema: img2imgSchema(),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		in img2imgInput,
	) (*mcp.CallToolResult, generationOutput, error) {
		log.Debug("tool called",
			zap.String("tool", "img2img"),
			zap.String("model", in.Model),
			zap.String("forge_preset", in.ForgePreset),
			zap.Strings("vae_and_text_models", in.VAEAndTextModels),
			zap.String("init_image_url", in.InitImageURL),
			zap.String("sampler", in.SamplingMethod),
			zap.String("scheduler", in.ScheduleType),
			zap.Int("steps", in.SamplingSteps),
			zap.Int("width", in.Width),
			zap.Int("height", in.Height),
			zap.Float64("cfg_scale", in.CFGScale),
			zap.Float64("denoising_strength", in.DenoisingStrength),
			zap.Int("seed", in.Seed),
			zap.Bool("enable_hr", in.EnableHR),
			zap.Float64("hr_scale", in.HRScale),
			zap.String("hr_upscaler", in.HRUpscaler),
			zap.Int("hr_second_pass_steps", in.HRSecondPassSteps),
			zap.Float64("hr_cfg", in.HRCFGScale),
		)

		initImage, err := client.FetchImage(ctx, in.InitImageURL)
		if err != nil {
			log.Error("img2img failed to fetch init image",
				zap.String("init_image_url", in.InitImageURL),
				zap.Error(err),
			)

			return nil, generationOutput{}, fmt.Errorf("img2img: fetch init image: %w", err)
		}

		log.Info("img2img generating synchronously",
			zap.String("model", in.Model),
			zap.Int("steps", in.SamplingSteps),
			zap.Int("width", in.Width),
			zap.Int("height", in.Height),
		)

		images, err := client.Img2Img(ctx, sdwebui.Img2ImgRequest{
			Checkpoint:             in.Model,
			ForgePreset:            in.ForgePreset,
			ForgeAdditionalModules: in.VAEAndTextModels,

			InitImageData: initImage,

			Prompt:         in.Prompt,
			NegativePrompt: in.NegativePrompt,
			Steps:          in.SamplingSteps,
			Width:          in.Width,
			Height:         in.Height,
			Seed:           in.Seed,
			CFGScale:       in.CFGScale,
			SamplerName:    in.SamplingMethod,
			Scheduler:      in.ScheduleType,

			DenoisingStrength: in.DenoisingStrength,

			EnableHR:          in.EnableHR,
			HRScale:           in.HRScale,
			HRUpscaler:        in.HRUpscaler,
			HRSecondPassSteps: in.HRSecondPassSteps,
			HRCFGScale:        in.HRCFGScale,
		})
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				log.Warn("img2img aborted: request context cancelled before completion",
					zap.Error(err),
					zap.String("ctx_err", ctxErr.Error()),
				)
			} else {
				log.Error("img2img generation failed", zap.Error(err))
			}

			return nil, generationOutput{}, fmt.Errorf("img2img: %w", err)
		}

		log.Info("img2img generation finished", zap.Int("images", len(images)))

		return publishImages(ctx, log, "img2img", images, uploader, shortenerClient)
	})
}

func img2imgSchema() *jsonschema.Schema {
	s, err := jsonschema.For[img2imgInput](nil)
	if err != nil {
		panic(fmt.Sprintf("img2img: infer input schema: %v", err))
	}

	setDefault(s.Properties, "negative_prompt", "")
	setDefault(s.Properties, "steps", 20)
	setDefault(s.Properties, "width", 512)
	setDefault(s.Properties, "height", 512)
	setDefault(s.Properties, "seed", -1)
	setDefault(s.Properties, "cfg_scale", 7.0)
	setDefault(s.Properties, "denoising_strength", 0.75)
	setDefault(s.Properties, "sampler_name", defaultSampler)
	setDefault(s.Properties, "scheduler", defaultScheduler)

	return s
}
