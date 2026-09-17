package present

import (
	"testing"
	"time"

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

func TestReviewsWithDetails(t *testing.T) {
	reviews := []store.Review{
		{
			ID:           5,
			Model:        "m",
			Rating:       8,
			Comment:      "nice",
			Prompt:       "a cat",
			ImageURL:     "https://cdn.example.com/a.png",
			AgentComment: "solid\nlighting",
		},
	}

	want := `# Reviews

Average: 8.0/10 (1 review)

## m

Average: 8.0/10 (1 review)

- 8/10 (id 5): nice
    - Image: https://cdn.example.com/a.png
    - Agent: solid lighting
`

	if got := Reviews(reviews); got != want {
		t.Errorf("Reviews() =\n%s\nwant:\n%s", got, want)
	}
}

func TestModelReviews(t *testing.T) {
	if got := ModelReviews("m", nil); got != "No reviews yet for m." {
		t.Errorf("ModelReviews(nil) = %q, want an empty notice", got)
	}

	reviews := []store.Review{
		{ID: 1, Model: "m", Rating: 4, Comment: "first"},
		{ID: 2, Model: "m", Rating: 6, Comment: "second", ImageURL: "https://cdn.example.com/b.png"},
	}

	want := `# Reviews for m

Average: 5.0/10 (2 reviews)

- 4/10 (id 1): first
- 6/10 (id 2): second
    - Image: https://cdn.example.com/b.png
`

	if got := ModelReviews("m", reviews); got != want {
		t.Errorf("ModelReviews() =\n%s\nwant:\n%s", got, want)
	}
}

func TestReview(t *testing.T) {
	review := store.Review{
		ID:           5,
		Model:        "m",
		Rating:       8,
		Comment:      "nice",
		Prompt:       "a cat\non a mat",
		ImageURL:     "https://cdn.example.com/a.png",
		AgentComment: "solid\nlighting",
		CreatedAt:    time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
	}

	want := `# Review 5

- Model: m
- Rating: 8/10
- Comment: nice
- Image: https://cdn.example.com/a.png
- Agent comment: solid lighting
- Created: 2026-09-17T12:00:00Z

## Prompt

` + "```\na cat\non a mat\n```\n"

	if got := Review(review); got != want {
		t.Errorf("Review() =\n%s\nwant:\n%s", got, want)
	}
}

func TestReviewWithoutDetails(t *testing.T) {
	review := store.Review{ID: 1, Model: "m", Rating: 3, Comment: "meh", CreatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)}

	want := `# Review 1

- Model: m
- Rating: 3/10
- Comment: meh
- Created: 2026-09-17T12:00:00Z
`

	if got := Review(review); got != want {
		t.Errorf("Review() =\n%s\nwant:\n%s", got, want)
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
