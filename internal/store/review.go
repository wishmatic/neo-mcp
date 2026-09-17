package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrReviewNotFound = errors.New("review not found")

const (
	minRating = 1
	maxRating = 10
)

const reviewColumns = `id, model, rating, comment, prompt, image_url, agent_comment, created_at`

type Review struct {
	ID           int64
	Model        string
	Rating       int
	Comment      string
	Prompt       string
	ImageURL     string
	AgentComment string
	CreatedAt    time.Time
}

// ReviewInput carries every field of a review. Only Model, Rating, and Comment are required.
type ReviewInput struct {
	Model        string
	Rating       int
	Comment      string
	Prompt       string
	ImageURL     string
	AgentComment string
}

func (c *Client) AddReview(ctx context.Context, model string, rating int, comment string) (Review, error) {
	return c.AddReviewDetails(ctx, ReviewInput{Model: model, Rating: rating, Comment: comment})
}

func (c *Client) AddReviewDetails(ctx context.Context, in ReviewInput) (Review, error) {
	in.Model = strings.TrimSpace(in.Model)
	in.Comment = strings.TrimSpace(in.Comment)
	in.Prompt = strings.TrimSpace(in.Prompt)
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	in.AgentComment = strings.TrimSpace(in.AgentComment)

	if err := validateModel(in.Model); err != nil {
		return Review{}, err
	}

	if in.Rating < minRating || in.Rating > maxRating {
		return Review{}, fmt.Errorf("store: rating %d is outside the %d-%d range", in.Rating, minRating, maxRating)
	}

	if err := validateComment(in.Comment); err != nil {
		return Review{}, err
	}

	if err := validateOptionalText("prompt", in.Prompt, MaxPromptLength); err != nil {
		return Review{}, err
	}

	if err := validateImageURL(in.ImageURL); err != nil {
		return Review{}, err
	}

	if err := validateOptionalText("agent comment", in.AgentComment, MaxAgentCommentLength); err != nil {
		return Review{}, err
	}

	createdAt := time.Now().UTC()

	result, err := c.db.ExecContext(
		ctx,
		`INSERT INTO reviews (model, rating, comment, prompt, image_url, agent_comment, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.Model, in.Rating, in.Comment, in.Prompt, in.ImageURL, in.AgentComment, formatTime(createdAt),
	)
	if err != nil {
		return Review{}, fmt.Errorf("store: insert review: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Review{}, fmt.Errorf("store: new review id: %w", err)
	}

	return Review{
		ID:           id,
		Model:        in.Model,
		Rating:       in.Rating,
		Comment:      in.Comment,
		Prompt:       in.Prompt,
		ImageURL:     in.ImageURL,
		AgentComment: in.AgentComment,
		CreatedAt:    createdAt,
	}, nil
}

func (c *Client) Reviews(ctx context.Context) ([]Review, error) {
	return c.queryReviews(ctx, `SELECT `+reviewColumns+` FROM reviews ORDER BY model COLLATE NOCASE, id`)
}

func (c *Client) ReviewsByModel(ctx context.Context, model string) ([]Review, error) {
	model = strings.TrimSpace(model)

	if err := validateModel(model); err != nil {
		return nil, err
	}

	return c.queryReviews(ctx, `SELECT `+reviewColumns+` FROM reviews WHERE model = ? COLLATE NOCASE ORDER BY id`, model)
}

func (c *Client) Review(ctx context.Context, id int64) (Review, error) {
	review, err := scanReview(c.db.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM reviews WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Review{}, fmt.Errorf("store: review %d: %w", id, ErrReviewNotFound)
	}

	if err != nil {
		return Review{}, err
	}

	return review, nil
}

func (c *Client) DeleteReview(ctx context.Context, id int64) (Review, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, fmt.Errorf("store: begin delete transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	review, err := scanReview(tx.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM reviews WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Review{}, fmt.Errorf("store: review %d: %w", id, ErrReviewNotFound)
	}

	if err != nil {
		return Review{}, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM reviews WHERE id = ?`, id); err != nil {
		return Review{}, fmt.Errorf("store: delete review %d: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return Review{}, fmt.Errorf("store: commit review delete: %w", err)
	}

	return review, nil
}

func (c *Client) queryReviews(ctx context.Context, query string, args ...any) ([]Review, error) {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query reviews: %w", err)
	}

	defer rows.Close()

	reviews := make([]Review, 0)

	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			return nil, err
		}

		reviews = append(reviews, review)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read reviews: %w", err)
	}

	return reviews, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReview(row rowScanner) (Review, error) {
	var (
		review    Review
		createdAt string
	)

	if err := row.Scan(
		&review.ID,
		&review.Model,
		&review.Rating,
		&review.Comment,
		&review.Prompt,
		&review.ImageURL,
		&review.AgentComment,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Review{}, err
		}

		return Review{}, fmt.Errorf("store: scan review: %w", err)
	}

	parsed, err := parseTime(createdAt)
	if err != nil {
		return Review{}, err
	}

	review.CreatedAt = parsed

	return review, nil
}
