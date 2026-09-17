package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()

	client, err := New(filepath.Join(t.TempDir(), "neo.db"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	return client
}

func tableExists(t *testing.T, client *Client, name string) bool {
	t.Helper()

	var count int

	err := client.db.QueryRowContext(
		context.Background(),
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}

	return count == 1
}

func TestNewCreatesDatabaseAndSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neo.db")

	client, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat database: %v", err)
	}

	for _, table := range []string{"reviews", "examples"} {
		if !tableExists(t, client, table) {
			t.Errorf("table %q is missing", table)
		}
	}
}

func TestNewRejectsEmptyPath(t *testing.T) {
	client, err := New("")
	if err == nil {
		_ = client.Close()

		t.Fatal("New(\"\") error = nil, want an error")
	}
}

func TestNewCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "neo.db")

	client, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat database: %v", err)
	}
}

func TestNewFailsWhenParentIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")

	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	if _, err := New(filepath.Join(blocker, "neo.db")); err == nil {
		t.Fatal("New() error = nil, want an error")
	}
}

func TestNewReopensExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neo.db")

	first, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := first.AddReview(context.Background(), "m", 5, "ok"); err != nil {
		t.Fatalf("AddReview() error: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	second, err := New(path)
	if err != nil {
		t.Fatalf("reopen error: %v", err)
	}

	t.Cleanup(func() { _ = second.Close() })

	reviews, err := second.Reviews(context.Background())
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 1 || reviews[0].Comment != "ok" {
		t.Fatalf("reviews = %+v, want the row written before the reopen", reviews)
	}
}

func TestClosePreventsFurtherUse(t *testing.T) {
	client := newTestClient(t)

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if _, err := client.AddReview(context.Background(), "m", 5, "ok"); err == nil {
		t.Fatal("AddReview() after Close() error = nil, want an error")
	}
}

func TestDSN(t *testing.T) {
	got := dsn("/tmp/neo.db")
	want := "/tmp/neo.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

	if got != want {
		t.Fatalf("dsn() = %q, want %q", got, want)
	}

	if strings.HasPrefix(got, "file:") {
		t.Errorf("dsn() = %q, want no file: prefix", got)
	}
}

func TestNewEnablesWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neo.db")

	client, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	t.Cleanup(func() { _ = raw.Close() })

	var mode string
	if err := raw.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}

	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestNewAddsMissingTablesToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neo.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	if _, err := raw.Exec(`CREATE TABLE reviews (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		model      TEXT    NOT NULL,
		rating     INTEGER NOT NULL,
		comment    TEXT    NOT NULL,
		created_at TEXT    NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy reviews table: %v", err)
	}

	if _, err := raw.Exec(
		`INSERT INTO reviews (model, rating, comment, created_at) VALUES ('legacy', 5, 'kept', '2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert legacy review: %v", err)
	}

	if err := raw.Close(); err != nil {
		t.Fatalf("close raw database: %v", err)
	}

	client, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	if !tableExists(t, client, "examples") {
		t.Error("examples table was not created")
	}

	reviews, err := client.Reviews(context.Background())
	if err != nil {
		t.Fatalf("Reviews() error: %v", err)
	}

	if len(reviews) != 1 || reviews[0].Model != "legacy" {
		t.Fatalf("reviews = %+v, want the pre-existing row untouched", reviews)
	}
}
