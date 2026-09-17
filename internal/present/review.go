package present

import (
	"fmt"
	"strings"

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
			fmt.Fprintf(&b, "\n- %d/10 (id %d): %s", review.Rating, review.ID, review.Comment)
		}

		b.WriteString("\n")
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
