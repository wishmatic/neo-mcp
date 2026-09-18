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

	if !tableExists(t, client, "examples") {
		t.Error("examples table is missing")
	}

	if tableExists(t, client, "reviews") {
		t.Error("reviews table should no longer be created")
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

	meta := ExampleMeta{
		Model: "m",
		Tool:  "txt2img",
		Query: `{"prompt":"a cat"}`,
		URL:   "https://cdn.example.com/a.png",
	}

	if _, err := first.SaveExample(context.Background(), meta, 16); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	second, err := New(path)
	if err != nil {
		t.Fatalf("reopen error: %v", err)
	}

	t.Cleanup(func() { _ = second.Close() })

	examples, err := second.RandomExamples(context.Background(), "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 || examples[0].URL != meta.URL {
		t.Fatalf("examples = %+v, want the row written before the reopen", examples)
	}
}

func TestClosePreventsFurtherUse(t *testing.T) {
	client := newTestClient(t)

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	meta := ExampleMeta{
		Model: "m",
		Tool:  "txt2img",
		Query: `{"prompt":"a cat"}`,
		URL:   "https://cdn.example.com/a.png",
	}

	if _, err := client.SaveExample(context.Background(), meta, 16); err == nil {
		t.Fatal("SaveExample() after Close() error = nil, want an error")
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

func TestNewAddsExamplesNSFWColumnToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neo.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	if _, err := raw.Exec(`CREATE TABLE examples (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		model      TEXT NOT NULL,
		tool       TEXT NOT NULL,
		query      TEXT NOT NULL,
		url        TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy examples table: %v", err)
	}

	if _, err := raw.Exec(
		`INSERT INTO examples (model, tool, query, url, created_at)
		 VALUES ('m', 'txt2img', '{}', 'https://cdn.example.com/a.png', '2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert legacy example: %v", err)
	}

	if err := raw.Close(); err != nil {
		t.Fatalf("close raw database: %v", err)
	}

	client, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	examples, err := client.RandomExamples(context.Background(), "m", 1)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 {
		t.Fatalf("examples = %d, want the pre-existing row", len(examples))
	}

	if examples[0].NSFW {
		t.Error("legacy example read back as NSFW, want non-NSFW")
	}
}

func TestNewAddsMissingTablesToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neo.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	if _, err := raw.Exec(`CREATE TABLE reviews (id INTEGER PRIMARY KEY, model TEXT NOT NULL)`); err != nil {
		t.Fatalf("create legacy reviews table: %v", err)
	}

	if _, err := raw.Exec(`INSERT INTO reviews (model) VALUES ('legacy')`); err != nil {
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

	var model string
	if err := client.db.QueryRow(`SELECT model FROM reviews`).Scan(&model); err != nil {
		t.Fatalf("read legacy reviews row: %v", err)
	}

	if model != "legacy" {
		t.Fatalf("legacy model = %q, want the pre-existing row untouched", model)
	}
}
