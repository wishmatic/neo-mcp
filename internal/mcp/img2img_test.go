package mcp

import (
	"context"
	"image"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/format"
	"github.com/wishmatic/neo-mcp/internal/generation"
)

func TestInitImageSize(t *testing.T) {
	tests := []struct {
		name       string
		in         generationInput
		initWidth  int
		initHeight int
		wantWidth  int
		wantHeight int
		wantErr    bool
	}{
		{name: "neither set takes the init image's size", initWidth: 768, initHeight: 512, wantWidth: 768, wantHeight: 512},
		{name: "neither set snaps to the sampling grid", initWidth: 1003, initHeight: 667, wantWidth: 1000, wantHeight: 664},
		{name: "both set are kept", in: generationInput{Width: 512, Height: 1024}, initWidth: 768, initHeight: 512, wantWidth: 512, wantHeight: 1024},
		{name: "width alone keeps the aspect ratio", in: generationInput{Width: 1536}, initWidth: 768, initHeight: 512, wantWidth: 1536, wantHeight: 1024},
		{name: "height alone keeps the aspect ratio and snaps", in: generationInput{Height: 256}, initWidth: 1003, initHeight: 667, wantWidth: 384, wantHeight: 256},
		{name: "a tiny image still gets the grid minimum", in: generationInput{Height: 1}, initWidth: 1, initHeight: 4096, wantWidth: 8, wantHeight: 1},
		{name: "an unreadable init image errors", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initImage := []byte("not an image")
			if !tt.wantErr {
				encoded, err := format.Encode(image.NewNRGBA(image.Rect(0, 0, tt.initWidth, tt.initHeight)), format.PNG)
				if err != nil {
					t.Fatalf("encode init image: %v", err)
				}

				initImage = encoded
			}

			width, height, err := initImageSize(tt.in, initImage)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("initImageSize() error = nil, want an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("initImageSize() error: %v", err)
			}

			if width != tt.wantWidth || height != tt.wantHeight {
				t.Errorf("initImageSize() = %dx%d, want %dx%d", width, height, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestImg2ImgSchemaLeavesDimensionsUnset(t *testing.T) {
	s := img2imgSchema(format.Default)

	for _, field := range []string{"width", "height"} {
		if s.Properties[field].Default != nil {
			t.Errorf("img2img %s default = %s, want none so the init image sizes it", field, s.Properties[field].Default)
		}

		if txt2imgSchema(format.Default).Properties[field].Default == nil {
			t.Errorf("txt2img %s default = nil, want the 512 the schema documents", field)
		}
	}
}

func TestImg2ImgCallToolTakesSizeFromInitImage(t *testing.T) {
	log := &requestLog{}
	srv := newImg2ImgServer(t, log)

	callImg2Img(t, srv, map[string]any{
		"model":              "nai-diffusion-5-full",
		"prompt":             "a cat",
		"forge_preset":       "",
		"init_image_url":     newSizedInitImageURL(t, 320, 640),
		"denoising_strength": 0.6,
	})

	params := paramsOf(t, singleRequestBody(t, log))

	width, height := numberField(t, params, "width"), numberField(t, params, "height")
	if width != 320 || height != 640 {
		t.Errorf("dimensions = %vx%v, want the init image's 320x640", width, height)
	}
}

func TestImg2ImgCallToolKeepsRequestedSize(t *testing.T) {
	log := &requestLog{}
	srv := newImg2ImgServer(t, log)

	callImg2Img(t, srv, map[string]any{
		"model":              "nai-diffusion-5-full",
		"prompt":             "a cat",
		"forge_preset":       "",
		"init_image_url":     newSizedInitImageURL(t, 1024, 1024),
		"denoising_strength": 0.6,
		"width":              832,
		"height":             1216,
	})

	params := paramsOf(t, singleRequestBody(t, log))

	width, height := numberField(t, params, "width"), numberField(t, params, "height")
	if width != 832 || height != 1216 {
		t.Errorf("dimensions = %vx%v, want the requested 832x1216", width, height)
	}
}

func TestImg2ImgForgeCallToolTakesSizeFromInitImage(t *testing.T) {
	log := &requestLog{}
	backend := newForgeBackend(t, log)

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: generation.New(backend, nil),
		Store:     newTestStore(t),
		Resolver:  newResolver(t),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	callImg2Img(t, srv, map[string]any{
		"model":              "sd_xl_base_1.0.safetensors",
		"prompt":             "a cat",
		"forge_preset":       "",
		"init_image_url":     newSizedInitImageURL(t, 1003, 667),
		"denoising_strength": 0.6,
	})

	body := singleRequestBody(t, log)

	width, height := numberField(t, body, "width"), numberField(t, body, "height")
	if width != 1000 || height != 664 {
		t.Errorf("dimensions = %vx%v, want the init image's 1003x667 on the 8px grid", width, height)
	}
}

func newImg2ImgServer(t *testing.T, log *requestLog) *mcp.Server {
	t.Helper()

	backend := newNovelAIBackend(t, log)

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: generation.New(nil, backend),
		Store:     newTestStore(t),
		Resolver:  newResolver(t),
		NovelAI:   backend,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return srv
}

func callImg2Img(t *testing.T, srv *mcp.Server, arguments map[string]any) {
	t.Helper()

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "img2img",
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				t.Fatalf("CallTool() tool error: %s", text.Text)
			}
		}

		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}
}

func singleRequestBody(t *testing.T, log *requestLog) map[string]any {
	t.Helper()

	_, bodies := log.snapshot()
	if len(bodies) != 1 {
		t.Fatalf("requests = %d, want 1", len(bodies))
	}

	return decodeJSONBody(t, bodies[0])
}
