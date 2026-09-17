package present

import (
	"testing"

	"github.com/wishmatic/neo-mcp/internal/store"
)

func TestReviews(t *testing.T) {
	tests := []struct {
		name    string
		reviews []store.Review
		want    string
	}{
		{
			name: "empty",
			want: "No reviews yet.",
		},
		{
			name: "grouped by model",
			reviews: []store.Review{
				{ID: 3, Model: "nai-diffusion-5-full", Rating: 9, Comment: "Superb hands"},
				{ID: 4, Model: "nai-diffusion-5-full", Rating: 8, Comment: "A little soft"},
				{ID: 1, Model: "sd_xl_base_1.0.safetensors", Rating: 6, Comment: "Weak textures"},
			},
			want: `# Reviews

Average: 7.7/10 (3 reviews)

## nai-diffusion-5-full

Average: 8.5/10 (2 reviews)

- 9/10 (id 3): Superb hands
- 8/10 (id 4): A little soft

## sd_xl_base_1.0.safetensors

Average: 6.0/10 (1 review)

- 6/10 (id 1): Weak textures
`,
		},
		{
			name: "case insensitive grouping",
			reviews: []store.Review{
				{ID: 1, Model: "a", Rating: 4, Comment: "first"},
				{ID: 2, Model: "A", Rating: 6, Comment: "second"},
				{ID: 3, Model: "B", Rating: 5, Comment: "third"},
			},
			want: `# Reviews

Average: 5.0/10 (3 reviews)

## a

Average: 5.0/10 (2 reviews)

- 4/10 (id 1): first
- 6/10 (id 2): second

## B

Average: 5.0/10 (1 review)

- 5/10 (id 3): third
`,
		},
		{
			name: "metacharacters and multi-byte comments",
			reviews: []store.Review{
				{ID: 7, Model: "m *x*", Rating: 10, Comment: "猫 | dog #1 <b> - great"},
			},
			want: `# Reviews

Average: 10.0/10 (1 review)

## m *x*

Average: 10.0/10 (1 review)

- 10/10 (id 7): 猫 | dog #1 <b> - great
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reviews(tt.reviews); got != tt.want {
				t.Errorf("Reviews() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestReviewAverage(t *testing.T) {
	if got := ReviewAverage(nil); got != 0 {
		t.Errorf("ReviewAverage(nil) = %v, want 0", got)
	}

	reviews := []store.Review{
		{Rating: 9},
		{Rating: 8},
		{Rating: 6},
	}

	if got := ReviewAverage(reviews); got != 23.0/3 {
		t.Errorf("ReviewAverage() = %v, want %v", got, 23.0/3)
	}
}
