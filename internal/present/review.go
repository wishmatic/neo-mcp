package present

import (
	"fmt"
	"strings"
	"time"

	"github.com/wishmatic/neo-mcp/internal/store"
)

type reviewGroup struct {
	model   string
	reviews []store.Review
}

func Reviews(reviews []store.Review) string {
	if len(reviews) == 0 {
		return "No reviews yet."
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Reviews\n\nAverage: %.1f/10 (%s)\n", ReviewAverage(reviews), reviewCount(len(reviews)))

	for _, group := range groupReviewsByModel(reviews) {
		fmt.Fprintf(
			&b,
			"\n## %s\n\nAverage: %.1f/10 (%s)\n",
			group.model, ReviewAverage(group.reviews), reviewCount(len(group.reviews)),
		)

		for _, review := range group.reviews {
			writeReview(&b, review)
		}

		b.WriteString("\n")
	}

	return b.String()
}

func ModelReviews(model string, reviews []store.Review) string {
	if len(reviews) == 0 {
		return fmt.Sprintf("No reviews yet for %s.", model)
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Reviews for %s\n\nAverage: %.1f/10 (%s)\n", model, ReviewAverage(reviews), reviewCount(len(reviews)))

	for _, review := range reviews {
		writeReview(&b, review)
	}

	b.WriteString("\n")

	return b.String()
}

func Review(review store.Review) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Review %d\n\n", review.ID)
	fmt.Fprintf(&b, "- Model: %s\n", review.Model)
	fmt.Fprintf(&b, "- Rating: %d/10\n", review.Rating)
	fmt.Fprintf(&b, "- Comment: %s\n", collapse(review.Comment))

	if review.ImageURL != "" {
		fmt.Fprintf(&b, "- Image: %s\n", review.ImageURL)
	}

	if review.AgentComment != "" {
		fmt.Fprintf(&b, "- Agent comment: %s\n", collapse(review.AgentComment))
	}

	fmt.Fprintf(&b, "- Created: %s\n", review.CreatedAt.UTC().Format(time.RFC3339))

	if review.Prompt != "" {
		fmt.Fprintf(&b, "\n## Prompt\n\n```\n%s\n```\n", review.Prompt)
	}

	return b.String()
}

func ReviewAverage(reviews []store.Review) float64 {
	if len(reviews) == 0 {
		return 0
	}

	total := 0
	for _, review := range reviews {
		total += review.Rating
	}

	return float64(total) / float64(len(reviews))
}

func writeReview(b *strings.Builder, review store.Review) {
	fmt.Fprintf(b, "\n- %d/10 (id %d): %s", review.Rating, review.ID, review.Comment)

	if review.ImageURL != "" {
		fmt.Fprintf(b, "\n    - Image: %s", review.ImageURL)
	}

	if review.AgentComment != "" {
		fmt.Fprintf(b, "\n    - Agent: %s", collapse(review.AgentComment))
	}
}

func collapse(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func reviewCount(count int) string {
	if count == 1 {
		return "1 review"
	}

	return fmt.Sprintf("%d reviews", count)
}

// Relies on Reviews ordering rows by model, so equal models are adjacent.
func groupReviewsByModel(reviews []store.Review) []reviewGroup {
	groups := make([]reviewGroup, 0, len(reviews))

	for _, review := range reviews {
		if len(groups) == 0 || !strings.EqualFold(groups[len(groups)-1].model, review.Model) {
			groups = append(groups, reviewGroup{model: review.Model})
		}

		last := len(groups) - 1
		groups[last].reviews = append(groups[last].reviews, review)
	}

	return groups
}
