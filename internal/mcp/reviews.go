package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/present"
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

	Prompt       string `json:"prompt,omitempty" jsonschema:"optional: the exact prompt used for the generation being reviewed; only get_review returns it, get_reviews never does"`
	ImageURL     string `json:"image_url,omitempty" jsonschema:"optional: the absolute http(s) URL of an image the review is about; omit it when you do not have a URL"`
	AgentComment string `json:"agent_comment,omitempty" jsonschema:"optional: your own notes about the model, beyond the one-line comment"`
}

type addReviewOutput struct {
	ID           int64  `json:"id" jsonschema:"the new review's id, which get_review and delete_review take"`
	Model        string `json:"model" jsonschema:"the model the review is about"`
	Rating       int    `json:"rating" jsonschema:"the recorded rating"`
	Comment      string `json:"comment" jsonschema:"the recorded commentary"`
	Prompt       string `json:"prompt,omitempty" jsonschema:"the recorded prompt, when one was given"`
	ImageURL     string `json:"image_url,omitempty" jsonschema:"the recorded image URL, when one was given"`
	AgentComment string `json:"agent_comment,omitempty" jsonschema:"the recorded agent notes, when any were given"`
}

type getReviewsInput struct {
	Model string `json:"model,omitempty" jsonschema:"optional: only return reviews for this model: a Forge checkpoint filename or a NovelAI model id. Omit to return reviews for every model."`
}

type getReviewsOutput struct {
	Model   string  `json:"model,omitempty" jsonschema:"the model the listing was filtered to, omitted when every model is returned"`
	Count   int     `json:"count" jsonschema:"number of reviews"`
	Average float64 `json:"average" jsonschema:"mean rating across the returned reviews, 0 when there are none"`
}

type getReviewInput struct {
	ID int64 `json:"id" jsonschema:"the exact id of the review to fetch, as listed by get_reviews or returned by add_review"`
}

type getReviewOutput struct {
	ID           int64  `json:"id" jsonschema:"the review's id"`
	Model        string `json:"model" jsonschema:"the model the review is about"`
	Rating       int    `json:"rating" jsonschema:"the recorded rating"`
	Comment      string `json:"comment" jsonschema:"the recorded commentary"`
	Prompt       string `json:"prompt,omitempty" jsonschema:"the recorded prompt, when one was given"`
	ImageURL     string `json:"image_url,omitempty" jsonschema:"the recorded image URL, when one was given"`
	AgentComment string `json:"agent_comment,omitempty" jsonschema:"the recorded agent notes, when any were given"`
	CreatedAt    string `json:"created_at" jsonschema:"when the review was recorded, in RFC 3339 form"`
}

type deleteReviewInput struct {
	ID int64 `json:"id" jsonschema:"id of the review to delete, as returned by add_review or listed by get_reviews"`
}

type deleteReviewOutput struct {
	ID           int64  `json:"id" jsonschema:"the deleted review's id"`
	Model        string `json:"model" jsonschema:"the model the deleted review was about"`
	Rating       int    `json:"rating" jsonschema:"the deleted rating"`
	Comment      string `json:"comment" jsonschema:"the deleted commentary"`
	Prompt       string `json:"prompt,omitempty" jsonschema:"the deleted prompt, when one was recorded"`
	ImageURL     string `json:"image_url,omitempty" jsonschema:"the deleted image URL, when one was recorded"`
	AgentComment string `json:"agent_comment,omitempty" jsonschema:"the deleted agent notes, when any were recorded"`
}

func registerReviews(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "add_review",
		Description: "Record a rating from 1 to 10 with one line of commentary about a model. Optionally attach the " +
			"prompt used, the URL of an image it produced, and your own notes. Reviews are stored locally; list them " +
			"with get_reviews, read one in full with get_review, and remove them with delete_review.",
		InputSchema: addReviewSchema(),
	}, h.addReview)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_reviews",
		Description: "Return saved model reviews as Markdown, including the average rating overall and for each model. " +
			"Pass a model to return only that model's reviews. Prompts are never included; use get_review for those.",
		InputSchema: getReviewsSchema(),
	}, h.getReviews)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_review",
		Description: "Return one saved model review in full, including any prompt, image URL, and agent notes, by its " +
			"exact id as listed by get_reviews.",
		InputSchema: getReviewSchema(),
	}, h.getReview)

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
		zap.Bool("has_prompt", in.Prompt != ""),
		zap.Bool("has_image_url", in.ImageURL != ""),
		zap.Bool("has_agent_comment", in.AgentComment != ""),
	)

	review, err := h.store.AddReviewDetails(ctx, store.ReviewInput{
		Model:        in.Model,
		Rating:       in.Rating,
		Comment:      in.Comment,
		Prompt:       in.Prompt,
		ImageURL:     in.ImageURL,
		AgentComment: in.AgentComment,
	})
	if err != nil {
		h.log.Error("add_review failed", zap.String("model", in.Model), zap.Error(err))

		return nil, addReviewOutput{}, fmt.Errorf("add_review: %w", err)
	}

	out := addReviewOutput{
		ID:           review.ID,
		Model:        review.Model,
		Rating:       review.Rating,
		Comment:      review.Comment,
		Prompt:       review.Prompt,
		ImageURL:     review.ImageURL,
		AgentComment: review.AgentComment,
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
	in getReviewsInput,
) (*mcp.CallToolResult, getReviewsOutput, error) {
	model := strings.TrimSpace(in.Model)

	h.log.Debug("tool called", zap.String("tool", "get_reviews"), zap.String("model", model))

	var (
		reviews []store.Review
		err     error
	)

	if model == "" {
		reviews, err = h.store.Reviews(ctx)
	} else {
		reviews, err = h.store.ReviewsByModel(ctx, model)
	}

	if err != nil {
		h.log.Error("get_reviews failed", zap.String("model", model), zap.Error(err))

		return nil, getReviewsOutput{}, fmt.Errorf("get_reviews: %w", err)
	}

	out := getReviewsOutput{Model: model, Count: len(reviews), Average: present.ReviewAverage(reviews)}

	h.log.Info("reviews listed", zap.String("model", model), zap.Int("count", out.Count), zap.Float64("average", out.Average))

	text := present.Reviews(reviews)
	if model != "" {
		text = present.ModelReviews(model, reviews)
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func (h *handlers) getReview(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in getReviewInput,
) (*mcp.CallToolResult, getReviewOutput, error) {
	h.log.Debug("tool called", zap.String("tool", "get_review"), zap.Int64("id", in.ID))

	review, err := h.store.Review(ctx, in.ID)
	if err != nil {
		if errors.Is(err, store.ErrReviewNotFound) {
			h.log.Warn("get_review found no such review", zap.Int64("id", in.ID))
		} else {
			h.log.Error("get_review failed", zap.Int64("id", in.ID), zap.Error(err))
		}

		return nil, getReviewOutput{}, fmt.Errorf("get_review: %w", err)
	}

	out := getReviewOutput{
		ID:           review.ID,
		Model:        review.Model,
		Rating:       review.Rating,
		Comment:      review.Comment,
		Prompt:       review.Prompt,
		ImageURL:     review.ImageURL,
		AgentComment: review.AgentComment,
		CreatedAt:    review.CreatedAt.UTC().Format(time.RFC3339),
	}

	h.log.Info("review fetched", zap.Int64("id", out.ID), zap.String("model", out.Model))

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: present.Review(review)}}}, out, nil
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
		ID:           review.ID,
		Model:        review.Model,
		Rating:       review.Rating,
		Comment:      review.Comment,
		Prompt:       review.Prompt,
		ImageURL:     review.ImageURL,
		AgentComment: review.AgentComment,
	}

	h.log.Info("review deleted", zap.Int64("id", out.ID), zap.String("model", out.Model))

	text := fmt.Sprintf("Deleted review %d: %s %d/10 - %s", out.ID, out.Model, out.Rating, out.Comment)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}
