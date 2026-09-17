package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAddReviewRoundTrip(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.AddReview(ctx, "nai-diffusion-5-full", 9, "Superb hands")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	if created.ID <= 0 {
		t.Fatalf("ID = %d, want a positive id", created.ID)
	}

	if delta := time.Since(created.CreatedAt); delta < 0 || delta > time.Minute {
		t.Errorf("CreatedAt = %s, want a recent timestamp", created.CreatedAt)
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 1 {
		t.Fatalf("reviews = %d, want 1", len(reviews))
	}

	got := reviews[0]
	if got.ID != created.ID || got.Model != "nai-diffusion-5-full" || got.Rating != 9 || got.Comment != "Superb hands" {
		t.Fatalf("review = %+v, want the stored review", got)
	}

	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt = %s, want %s", got.CreatedAt, created.CreatedAt)
	}
}

func TestAddReviewIDSareNotReused(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	first, err := client.AddReview(ctx, "m", 5, "one")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	second, err := client.AddReview(ctx, "m", 5, "two")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	if second.ID <= first.ID {
		t.Fatalf("ids = %d then %d, want increasing", first.ID, second.ID)
	}

	if _, err := client.DeleteReview(ctx, second.ID); err != nil {
		t.Fatalf("DeleteReview() error: %v", err)
	}

	third, err := client.AddReview(ctx, "m", 5, "three")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	if third.ID <= second.ID {
		t.Fatalf("id = %d after deleting %d, want a fresh id", third.ID, second.ID)
	}
}

func TestAddReviewRatingBounds(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for _, rating := range []int{1, 10} {
		if _, err := client.AddReview(ctx, "m", rating, "fine"); err != nil {
			t.Errorf("AddReview(rating=%d) error: %v", rating, err)
		}
	}

	for _, rating := range []int{0, 11, -1} {
		_, err := client.AddReview(ctx, "m", rating, "bad")
		if err == nil {
			t.Fatalf("AddReview(rating=%d) error = nil, want an error", rating)
		}

		if !strings.Contains(err.Error(), fmt.Sprint(rating)) {
			t.Errorf("error = %q, want it to name the rating", err)
		}
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 2 {
		t.Fatalf("reviews = %d, want only the two valid ratings", len(reviews))
	}
}

func TestAddReviewTrimsAndRequiresFields(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.AddReview(ctx, "  cat  ", 7, "  good  ")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	if created.Model != "cat" || created.Comment != "good" {
		t.Fatalf("review = %+v, want trimmed values", created)
	}

	tests := []struct {
		name    string
		model   string
		comment string
	}{
		{name: "empty model", model: "", comment: "ok"},
		{name: "whitespace model", model: "   ", comment: "ok"},
		{name: "empty comment", model: "m", comment: ""},
		{name: "whitespace comment", model: "m", comment: " \t "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := client.AddReview(ctx, tt.model, 5, tt.comment); err == nil {
				t.Fatal("AddReview() error = nil, want an error")
			}
		})
	}
}

func TestAddReviewRejectsMultiLineComment(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for _, comment := range []string{"one\ntwo", "one\rtwo", "one\ttwo", "one\x00two"} {
		if _, err := client.AddReview(ctx, "m", 5, comment); err == nil {
			t.Errorf("AddReview(comment=%q) error = nil, want an error", comment)
		}
	}

	if _, err := client.AddReview(ctx, "m", 5, "two  spaces"); err != nil {
		t.Errorf("AddReview() rejected a single-line comment: %v", err)
	}
}

func TestAddReviewLengthLimits(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.AddReview(ctx, "m", 5, strings.Repeat("é", MaxCommentLength)); err != nil {
		t.Errorf("AddReview() rejected a %d rune comment: %v", MaxCommentLength, err)
	}

	if _, err := client.AddReview(ctx, "m", 5, strings.Repeat("é", MaxCommentLength+1)); err == nil {
		t.Error("AddReview() accepted an over-long comment")
	}

	if _, err := client.AddReview(ctx, strings.Repeat("é", MaxModelLength), 5, "ok"); err != nil {
		t.Errorf("AddReview() rejected a %d rune model: %v", MaxModelLength, err)
	}

	if _, err := client.AddReview(ctx, strings.Repeat("é", MaxModelLength+1), 5, "ok"); err == nil {
		t.Error("AddReview() accepted an over-long model")
	}
}

func TestAddReviewDetailsRoundTrip(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.AddReviewDetails(ctx, ReviewInput{
		Model:        "m",
		Rating:       9,
		Comment:      "great",
		Prompt:       "a cat\non a mat",
		ImageURL:     "https://cdn.example.com/a.png",
		AgentComment: "handled fur well",
	})
	if err != nil {
		t.Fatalf("AddReviewDetails() error: %v", err)
	}

	got, err := client.Review(ctx, created.ID)
	if err != nil {
		t.Fatalf("Review() error: %v", err)
	}

	if got.Prompt != "a cat\non a mat" || got.ImageURL != "https://cdn.example.com/a.png" || got.AgentComment != "handled fur well" {
		t.Fatalf("review = %+v, want the recorded details", got)
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 1 || reviews[0].Prompt != got.Prompt || reviews[0].AgentComment != got.AgentComment {
		t.Fatalf("reviews = %+v, want the recorded details", reviews)
	}
}

func TestAddReviewWithoutDetailsLeavesThemEmpty(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.AddReview(ctx, "m", 5, "ok")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	if created.Prompt != "" || created.ImageURL != "" || created.AgentComment != "" {
		t.Fatalf("review = %+v, want empty details", created)
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 1 || reviews[0].Prompt != "" || reviews[0].ImageURL != "" || reviews[0].AgentComment != "" {
		t.Fatalf("reviews = %+v, want empty details", reviews)
	}
}

func TestAddReviewTrimsDetails(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.AddReviewDetails(ctx, ReviewInput{
		Model:        "  m  ",
		Rating:       5,
		Comment:      "  ok  ",
		Prompt:       "  a cat  ",
		ImageURL:     "  https://cdn.example.com/a.png  ",
		AgentComment: "  note  ",
	})
	if err != nil {
		t.Fatalf("AddReviewDetails() error: %v", err)
	}

	if created.Prompt != "a cat" || created.ImageURL != "https://cdn.example.com/a.png" || created.AgentComment != "note" {
		t.Fatalf("review = %+v, want trimmed details", created)
	}
}

func TestAddReviewImageURLValidation(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for _, raw := range []string{
		"example.com/a.png",
		"/a.png",
		"ftp://example.com/a.png",
		"data:image/png;base64,AAAA",
		"https://",
	} {
		in := ReviewInput{Model: "m", Rating: 5, Comment: "ok", ImageURL: raw}
		if _, err := client.AddReviewDetails(ctx, in); err == nil {
			t.Errorf("AddReviewDetails(image_url=%q) error = nil, want an error", raw)
		}
	}

	for _, raw := range []string{"http://example.com/a.png", "https://cdn.example.com/a.png?x=1"} {
		in := ReviewInput{Model: "m", Rating: 5, Comment: "ok", ImageURL: raw}
		if _, err := client.AddReviewDetails(ctx, in); err != nil {
			t.Errorf("AddReviewDetails(image_url=%q) error: %v", raw, err)
		}
	}
}

func TestAddReviewDetailTextValidation(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.AddReviewDetails(ctx, ReviewInput{
		Model: "m", Rating: 5, Comment: "ok", Prompt: strings.Repeat("é", MaxPromptLength),
	}); err != nil {
		t.Errorf("AddReviewDetails() rejected a %d rune prompt: %v", MaxPromptLength, err)
	}

	if _, err := client.AddReviewDetails(ctx, ReviewInput{
		Model: "m", Rating: 5, Comment: "ok", Prompt: strings.Repeat("é", MaxPromptLength+1),
	}); err == nil {
		t.Error("AddReviewDetails() accepted an over-long prompt")
	}

	if _, err := client.AddReviewDetails(ctx, ReviewInput{
		Model: "m", Rating: 5, Comment: "ok", AgentComment: strings.Repeat("é", MaxAgentCommentLength+1),
	}); err == nil {
		t.Error("AddReviewDetails() accepted an over-long agent comment")
	}

	if _, err := client.AddReviewDetails(ctx, ReviewInput{
		Model: "m", Rating: 5, Comment: "ok", AgentComment: "one\x00two",
	}); err == nil {
		t.Error("AddReviewDetails() accepted a control character in an agent comment")
	}
}

func TestReviewsByModel(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for _, review := range []struct{ model, comment string }{
		{model: "a", comment: "a1"},
		{model: "b", comment: "b1"},
		{model: "A", comment: "a2"},
	} {
		if _, err := client.AddReview(ctx, review.model, 5, review.comment); err != nil {
			t.Fatalf("AddReview() error: %v", err)
		}
	}

	reviews, err := client.ReviewsByModel(ctx, " a ")
	if err != nil {
		t.Fatalf("ReviewsByModel() error: %v", err)
	}

	if len(reviews) != 2 || reviews[0].Comment != "a1" || reviews[1].Comment != "a2" {
		t.Fatalf("reviews = %+v, want only model a's reviews in id order", reviews)
	}

	if _, err := client.ReviewsByModel(ctx, "   "); err == nil {
		t.Error("ReviewsByModel() error = nil, want an error for an empty model")
	}
}

func TestReviewByID(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.AddReview(ctx, "m", 7, "ok")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	got, err := client.Review(ctx, created.ID)
	if err != nil {
		t.Fatalf("Review() error: %v", err)
	}

	if got.ID != created.ID || got.Rating != 7 || got.Comment != "ok" {
		t.Fatalf("review = %+v, want the stored review", got)
	}

	if _, err := client.Review(ctx, created.ID+1); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("Review(unknown) error = %v, want ErrReviewNotFound", err)
	}
}

func TestReviewsEmpty(t *testing.T) {
	client := newTestClient(t)

	reviews, err := client.Reviews(context.Background())
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if reviews == nil {
		t.Fatal("Reviews() = nil, want an empty slice")
	}

	if len(reviews) != 0 {
		t.Fatalf("reviews = %d, want 0", len(reviews))
	}
}

func TestReviewsOrdering(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for _, review := range []struct {
		model   string
		comment string
	}{
		{model: "B", comment: "b-first"},
		{model: "a", comment: "a-first"},
		{model: "B", comment: "b-second"},
		{model: "A", comment: "a-second"},
	} {
		if _, err := client.AddReview(ctx, review.model, 5, review.comment); err != nil {
			t.Fatalf("AddReview() error: %v", err)
		}
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	got := make([]string, 0, len(reviews))
	for _, review := range reviews {
		got = append(got, review.Model+"/"+review.Comment)
	}

	want := []string{"a/a-first", "A/a-second", "B/b-first", "B/b-second"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestDeleteReview(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	first, err := client.AddReview(ctx, "m", 3, "keep")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	second, err := client.AddReview(ctx, "m", 8, "drop")
	if err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	deleted, err := client.DeleteReview(ctx, second.ID)
	if err != nil {
		t.Fatalf("DeleteReview() error: %v", err)
	}

	if deleted.ID != second.ID || deleted.Comment != "drop" || deleted.Rating != 8 {
		t.Fatalf("deleted = %+v, want the removed review", deleted)
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 1 || reviews[0].ID != first.ID || reviews[0].Comment != "keep" {
		t.Fatalf("reviews = %+v, want only the first review", reviews)
	}

	if _, err := client.DeleteReview(ctx, second.ID); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("second DeleteReview() error = %v, want ErrReviewNotFound", err)
	}
}

func TestDeleteReviewUnknownIDs(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for _, id := range []int64{0, -1, 4321} {
		if _, err := client.DeleteReview(ctx, id); !errors.Is(err, ErrReviewNotFound) {
			t.Errorf("DeleteReview(%d) error = %v, want ErrReviewNotFound", id, err)
		}
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 0 {
		t.Fatalf("reviews = %d, want 0", len(reviews))
	}
}

func TestAddReviewConcurrent(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	const (
		writers   = 8
		perWriter = 4
	)

	var (
		wg   sync.WaitGroup
		errs = make(chan error, writers*perWriter)
	)

	for writer := range writers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := range perWriter {
				if _, err := client.AddReview(ctx, "m", 5, fmt.Sprintf("w%d-%d", writer, i)); err != nil {
					errs <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent AddReview() error: %v", err)
	}

	reviews, err := client.Reviews(ctx)
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != writers*perWriter {
		t.Fatalf("reviews = %d, want %d", len(reviews), writers*perWriter)
	}
}
