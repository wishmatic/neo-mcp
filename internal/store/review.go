package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrReviewNotFound = errors.New("review not found")

const (
	minRating = 1
	maxRating = 10
)

type Review struct {
	ID        int64
	Model     string
	Rating    int
	Comment   string
	CreatedAt time.Time
}

func (c *Client) AddReview(ctx context.Context, model string, rating int, comment string) (Review, error) {
	model = strings.TrimSpace(model)
	comment = strings.TrimSpace(comment)

	if err := validateModel(model); err != nil {
		return Review{}, err
	}

	if rating < minRating || rating > maxRating {
		return Review{}, fmt.Errorf("store: rating %d is outside the %d-%d range", rating, minRating, maxRating)
	}

	if err := validateComment(comment); err != nil {
		return Review{}, err
	}

	createdAt := time.Now().UTC()

	result, err := c.db.ExecContext(
		ctx,
		`INSERT INTO reviews (model, rating, comment, created_at) VALUES (?, ?, ?, ?)`,
		model, rating, comment, formatTime(createdAt),
	)
	if err != nil {
		return Review{}, fmt.Errorf("store: insert review: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Review{}, fmt.Errorf("store: new review id: %w", err)
	}

	return Review{ID: id, Model: model, Rating: rating, Comment: comment, CreatedAt: createdAt}, nil
}

func (c *Client) Reviews(ctx context.Context) ([]Review, error) {
	rows, err := c.db.QueryContext(
		ctx,
		`SELECT id, model, rating, comment, created_at FROM reviews ORDER BY model COLLATE NOCASE, id`,
	)
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

func (c *Client) DeleteReview(ctx context.Context, id int64) (Review, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, fmt.Errorf("store: begin delete transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	review, err := scanReview(tx.QueryRowContext(
		ctx,
		`SELECT id, model, rating, comment, created_at FROM reviews WHERE id = ?`,
		id,
	))
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReview(row rowScanner) (Review, error) {
	var (
		review    Review
		createdAt string
	)

	if err := row.Scan(&review.ID, &review.Model, &review.Rating, &review.Comment, &createdAt); err != nil {
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

func validateModel(model string) error {
	if model == "" {
		return errors.New("store: model is required")
	}

	if utf8.RuneCountInString(model) > MaxModelLength {
		return fmt.Errorf("store: model is longer than %d characters", MaxModelLength)
	}

	return nil
}

func validateComment(comment string) error {
	if comment == "" {
		return errors.New("store: comment is required")
	}

	if utf8.RuneCountInString(comment) > MaxCommentLength {
		return fmt.Errorf("store: comment is longer than %d characters", MaxCommentLength)
	}

	for _, r := range comment {
		if unicode.IsControl(r) {
			return errors.New("store: comment must be a single line without control characters")
		}
	}

	return nil
}
