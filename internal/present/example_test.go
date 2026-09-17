package present

import (
	"testing"
	"time"

	"github.com/wishmatic/neo-mcp/internal/store"
)

func TestExamples(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		examples []store.Example
		want     string
	}{
		{
			name:  "empty",
			model: "m",
			want:  "No examples saved for m yet.",
		},
		{
			name:  "two examples",
			model: "nai-diffusion-5-full",
			examples: []store.Example{
				{
					ID: 1,
					ExampleMeta: store.ExampleMeta{
						Model: "nai-diffusion-5-full",
						Tool:  "txt2img",
						Query: `{"model":"nai-diffusion-5-full","prompt":"a cat","steps":28}`,
						URL:   "https://cdn.example.com/a.png",
					},
					CreatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
				},
				{
					ID: 2,
					ExampleMeta: store.ExampleMeta{
						Model: "nai-diffusion-5-full",
						Tool:  "img2img",
						Query: `{"prompt":"a dog","init_image_url":"https://example.com/dog.png","denoising_strength":0.75}`,
						URL:   "https://cdn.example.com/b.png",
					},
					CreatedAt: time.Date(2026, 9, 17, 12, 5, 0, 0, time.UTC),
				},
			},
			want: `# Examples for nai-diffusion-5-full

## Example 1 (txt2img)

` + "```json\n" + `{
  "model": "nai-diffusion-5-full",
  "prompt": "a cat",
  "steps": 28
}
` + "```" + `

- Saved: 2026-09-17T12:00:00Z
- Image: https://cdn.example.com/a.png

## Example 2 (img2img)

` + "```json\n" + `{
  "prompt": "a dog",
  "init_image_url": "https://example.com/dog.png",
  "denoising_strength": 0.75
}
` + "```" + `

- Saved: 2026-09-17T12:05:00Z
- Image: https://cdn.example.com/b.png
`,
		},
		{
			name:  "invalid query is rendered raw",
			model: "m",
			examples: []store.Example{
				{
					ID: 3,
					ExampleMeta: store.ExampleMeta{
						Model: "m",
						Tool:  "txt2img",
						Query: "not json",
						URL:   "https://cdn.example.com/c.png",
					},
					CreatedAt: time.Date(2026, 9, 17, 12, 10, 0, 0, time.UTC),
				},
			},
			want: `# Examples for m

## Example 1 (txt2img)

` + "```json\n" + `not json
` + "```" + `

- Saved: 2026-09-17T12:10:00Z
- Image: https://cdn.example.com/c.png
`,
		},
		{
			name:  "multi-byte and metacharacters",
			model: "m",
			examples: []store.Example{
				{
					ID: 4,
					ExampleMeta: store.ExampleMeta{
						Model: "m",
						Tool:  "txt2img",
						Query: `{"prompt":"猫 | dog #1 <b>\nsecond line","negative_prompt":"blurry"}`,
						URL:   "https://cdn.example.com/d.png",
					},
					CreatedAt: time.Date(2026, 9, 17, 12, 15, 0, 0, time.UTC),
				},
			},
			want: `# Examples for m

## Example 1 (txt2img)

` + "```json\n" + `{
  "prompt": "猫 | dog #1 <b>\nsecond line",
  "negative_prompt": "blurry"
}
` + "```" + `

- Saved: 2026-09-17T12:15:00Z
- Image: https://cdn.example.com/d.png
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Examples(tt.model, tt.examples); got != tt.want {
				t.Errorf("Examples() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
