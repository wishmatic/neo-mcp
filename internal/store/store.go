package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Exported so the MCP tool schemas advertise the same limits the store enforces.
const (
	MaxModelLength        = 200
	MaxCommentLength      = 500
	MaxPromptLength       = 4000
	MaxAgentCommentLength = 2000
	MaxImageURLLength     = 2048
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS reviews (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		model         TEXT    NOT NULL,
		rating        INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 10),
		comment       TEXT    NOT NULL,
		prompt        TEXT    NOT NULL DEFAULT '',
		image_url     TEXT    NOT NULL DEFAULT '',
		agent_comment TEXT    NOT NULL DEFAULT '',
		created_at    TEXT    NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS reviews_model_id_idx ON reviews (model, id)`,
	`CREATE TABLE IF NOT EXISTS examples (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		model      TEXT NOT NULL,
		tool       TEXT NOT NULL,
		query      TEXT NOT NULL,
		url        TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS examples_model_id_idx ON examples (model, id)`,
}

// Columns added to reviews after the initial release, applied to databases created before they existed.
var reviewDetailColumns = []struct{ name, definition string }{
	{"prompt", "TEXT NOT NULL DEFAULT ''"},
	{"image_url", "TEXT NOT NULL DEFAULT ''"},
	{"agent_comment", "TEXT NOT NULL DEFAULT ''"},
}

type Client struct {
	db *sql.DB
}

func New(path string) (*Client, error) {
	if path == "" {
		return nil, errors.New("store: empty database path")
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("store: create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("store: open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

func dsn(path string) string {
	return path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

func migrate(db *sql.DB) error {
	ctx := context.Background()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("store: ping database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin schema transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	for _, statement := range schema {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("store: apply schema: %w", err)
		}
	}

	if err := addReviewDetailColumns(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit schema: %w", err)
	}

	return nil
}

func addReviewDetailColumns(ctx context.Context, tx *sql.Tx) error {
	for _, column := range reviewDetailColumns {
		present, err := hasColumn(ctx, tx, "reviews", column.name)
		if err != nil {
			return err
		}

		if present {
			continue
		}

		statement := fmt.Sprintf(`ALTER TABLE reviews ADD COLUMN %s %s`, column.name, column.definition)

		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("store: add reviews.%s: %w", column.name, err)
		}
	}

	return nil
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, fmt.Errorf("store: read %s columns: %w", table, err)
	}

	defer rows.Close()

	for rows.Next() {
		var (
			position   int
			name       string
			kind       string
			notNull    int
			fallback   sql.NullString
			primaryKey int
		)

		if err := rows.Scan(&position, &name, &kind, &notNull, &fallback, &primaryKey); err != nil {
			return false, fmt.Errorf("store: scan %s column: %w", table, err)
		}

		if name == column {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("store: read %s columns: %w", table, err)
	}

	return false, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse timestamp %q: %w", value, err)
	}

	return parsed, nil
}
