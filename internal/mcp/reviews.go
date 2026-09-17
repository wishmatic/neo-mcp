package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
)

const (
	minReviewRating = 1
	maxReviewRating = 10
)

type addReviewInput struct {
	Model   string `json:"model" jsonschema:"the model being reviewed: a Forge checkpoint filename or a NovelAI model id"`
	Rating  int    `json:"rating" jsonschema:"score from 1 (worst) to 10 (best)"`
	Comment string `json:"comment" jsonschema:"one line of commentary explaining the rating"`
}

type addReviewOutput struct {
	ID      int64  `json:"id" jsonschema:"the new review's id, which delete_review takes"`
	Model   string `json:"model" jsonschema:"the model the review is about"`
	Rating  int    `json:"rating" jsonschema:"the recorded rating"`
	Comment string `json:"comment" jsonschema:"the recorded commentary"`
}

type getReviewsInput struct{}

type getReviewsOutput struct {
	Count   int     `json:"count" jsonschema:"number of reviews"`
	Average float64 `json:"average" jsonschema:"mean rating across all reviews, 0 when there are none"`
}

type deleteReviewInput struct {
	ID int64 `json:"id" jsonschema:"id of the review to delete, as returned by add_review or listed by get_reviews"`
}

type deleteReviewOutput struct {
	ID      int64  `json:"id" jsonschema:"the deleted review's id"`
	Model   string `json:"model" jsonschema:"the model the deleted review was about"`
	Rating  int    `json:"rating" jsonschema:"the deleted rating"`
	Comment string `json:"comment" jsonschema:"the deleted commentary"`
}

func registerReviews(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "add_review",
		Description: "Record a rating from 1 to 10 with one line of commentary about a model. Reviews are stored " +
			"locally; list them with get_reviews and remove them with delete_review.",
		InputSchema: addReviewSchema(),
	}, h.addReview)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_reviews",
		Description: "Return every saved model review as Markdown, including the average rating overall and for each " +
			"model.",
	}, h.getReviews)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_review",
		Description: "Delete a stored model review by its id, as returned by add_review or listed by get_reviews.",
		InputSchema: deleteReviewSchema(),
	}, h.deleteReview)
}

func (h *handlers) addReview(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in addReviewInput,
) (*mcp.CallToolResult, addReviewOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "add_review"),
		zap.String("model", in.Model),
		zap.Int("rating", in.Rating),
	)

	review, err := h.store.AddReview(ctx, in.Model, in.Rating, in.Comment)
	if err != nil {
		h.log.Error("add_review failed", zap.String("model", in.Model), zap.Error(err))

		return nil, addReviewOutput{}, fmt.Errorf("add_review: %w", err)
	}

	out := addReviewOutput{
		ID:      review.ID,
		Model:   review.Model,
		Rating:  review.Rating,
		Comment: review.Comment,
	}

	h.log.Info("review added",
		zap.Int64("id", out.ID),
		zap.String("model", out.Model),
		zap.Int("rating", out.Rating),
	)

	text := fmt.Sprintf("Added review %d for %s: %d/10 - %s", out.ID, out.Model, out.Rating, out.Comment)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func (h *handlers) getReviews(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ getReviewsInput,
) (*mcp.CallToolResult, getReviewsOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "get_reviews"))

	reviews, err := h.store.Reviews(ctx)
	if err != nil {
		h.log.Error("get_reviews failed", zap.Error(err))

		return nil, getReviewsOutput{}, fmt.Errorf("get_reviews: %w", err)
	}

	out := getReviewsOutput{Count: len(reviews), Average: averageRating(reviews)}

	h.log.Info("reviews listed", zap.Int("count", out.Count), zap.Float64("average", out.Average))

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: reviewsMarkdown(reviews)}}}, out, nil
}

func (h *handlers) deleteReview(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in deleteReviewInput,
) (*mcp.CallToolResult, deleteReviewOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "delete_review"), zap.Int64("id", in.ID))

	review, err := h.store.DeleteReview(ctx, in.ID)
	if err != nil {
		if errors.Is(err, store.ErrReviewNotFound) {
			h.log.Warn("delete_review found no such review", zap.Int64("id", in.ID))
		} else {
			h.log.Error("delete_review failed", zap.Int64("id", in.ID), zap.Error(err))
		}

		return nil, deleteReviewOutput{}, fmt.Errorf("delete_review: %w", err)
	}

	out := deleteReviewOutput{
		ID:      review.ID,
		Model:   review.Model,
		Rating:  review.Rating,
		Comment: review.Comment,
	}

	h.log.Info("review deleted", zap.Int64("id", out.ID), zap.String("model", out.Model))

	text := fmt.Sprintf("Deleted review %d: %s %d/10 - %s", out.ID, out.Model, out.Rating, out.Comment)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func addReviewSchema() *jsonschema.Schema {
	s, err := jsonschema.For[addReviewInput](nil)
	if err != nil {
		panic(fmt.Sprintf("add_review: infer input schema: %v", err))
	}

	s.Properties["rating"].Minimum = jsonschema.Ptr(float64(minReviewRating))
	s.Properties["rating"].Maximum = jsonschema.Ptr(float64(maxReviewRating))
	s.Properties["model"].MaxLength = jsonschema.Ptr(store.MaxModelLength)
	s.Properties["comment"].MaxLength = jsonschema.Ptr(store.MaxCommentLength)

	return s
}

func deleteReviewSchema() *jsonschema.Schema {
	s, err := jsonschema.For[deleteReviewInput](nil)
	if err != nil {
		panic(fmt.Sprintf("delete_review: infer input schema: %v", err))
	}

	return s
}
