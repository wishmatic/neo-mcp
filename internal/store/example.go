package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ExampleMeta struct {
	Model string
	Tool  string
	Query string
	URL   string
}

type Example struct {
	ID int64
	ExampleMeta
	CreatedAt time.Time
}

func (c *Client) SaveExample(ctx context.Context, meta ExampleMeta, retainPerModel int) (int64, error) {
	meta.Model = strings.TrimSpace(meta.Model)
	meta.Tool = strings.TrimSpace(meta.Tool)
	meta.URL = strings.TrimSpace(meta.URL)

	if err := validateModel(meta.Model); err != nil {
		return 0, err
	}

	if meta.Tool == "" {
		return 0, errors.New("store: example tool is required")
	}

	if meta.Query == "" {
		return 0, errors.New("store: example query is required")
	}

	if !json.Valid([]byte(meta.Query)) {
		return 0, errors.New("store: example query must be valid JSON")
	}

	if meta.URL == "" {
		return 0, errors.New("store: example url is required")
	}

	if retainPerModel < 1 {
		return 0, errors.New("store: example retention must be at least 1")
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: begin example transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO examples (model, tool, query, url, created_at) VALUES (?, ?, ?, ?, ?)`,
		meta.Model, meta.Tool, meta.Query, meta.URL, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return 0, fmt.Errorf("store: insert example: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM examples
		 WHERE model = ?
		   AND id NOT IN (SELECT id FROM examples WHERE model = ? ORDER BY id DESC LIMIT ?)`,
		meta.Model, meta.Model, retainPerModel,
	); err != nil {
		return 0, fmt.Errorf("store: prune examples: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store: commit example: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("store: new example id: %w", err)
	}

	return id, nil
}

func (c *Client) RandomExamples(ctx context.Context, model string, count int) ([]Example, error) {
	model = strings.TrimSpace(model)

	if model == "" {
		return nil, errors.New("store: model is required")
	}

	if count < 1 {
		return nil, errors.New("store: example count must be at least 1")
	}

	rows, err := c.db.QueryContext(
		ctx,
		`SELECT id, model, tool, query, url, created_at FROM examples WHERE model = ? ORDER BY RANDOM() LIMIT ?`,
		model, count,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query examples: %w", err)
	}

	defer rows.Close()

	examples := make([]Example, 0, count)

	for rows.Next() {
		var (
			example   Example
			createdAt string
		)

		if err := rows.Scan(
			&example.ID, &example.Model, &example.Tool, &example.Query, &example.URL, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("store: scan example: %w", err)
		}

		parsed, err := parseTime(createdAt)
		if err != nil {
			return nil, err
		}

		example.CreatedAt = parsed

		examples = append(examples, example)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read examples: %w", err)
	}

	return examples, nil
}
