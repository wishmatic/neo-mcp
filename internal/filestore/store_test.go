package filestore

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.uber.org/zap"
)

var keyPattern = regexp.MustCompile(`^i/(nsfw/)?\d{4}-\d{2}/[0-9a-f-]{36}\.(png|jpg|jxl|webp)$`)

func newTestClient(t *testing.T) (*Client, string) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "files")

	base, err := url.Parse("https://neo.example.com")
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	client, err := New(Config{Dir: dir, PublicBase: base}, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return client, dir
}

func upload(t *testing.T, client *Client, contentType string, nsfw bool) (string, string) {
	t.Helper()

	url, err := client.UploadFile(context.Background(), []byte("image-bytes"), contentType, nsfw)
	if err != nil {
		t.Fatalf("UploadFile() error: %v", err)
	}

	return url, strings.TrimPrefix(url, "https://neo.example.com/")
}

func TestNewCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "files")

	base, _ := url.Parse("https://neo.example.com")

	if _, err := New(Config{Dir: dir, PublicBase: base}, zap.NewNop()); err != nil {
		t.Fatalf("New() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	base, _ := url.Parse("https://neo.example.com")

	if _, err := New(Config{PublicBase: base}, zap.NewNop()); err == nil {
		t.Error("New() error = nil, want an error for an empty directory")
	}

	if _, err := New(Config{Dir: t.TempDir()}, zap.NewNop()); err == nil {
		t.Error("New() error = nil, want an error for a missing public base")
	}
}

func TestUploadAndGetRoundTrip(t *testing.T) {
	client, _ := newTestClient(t)

	url, key := upload(t, client, "image/png", false)

	if !keyPattern.MatchString(key) {
		t.Fatalf("key = %q, want a match for %s", key, keyPattern)
	}

	data, err := client.GetObject(context.Background(), key)
	if err != nil {
		t.Fatalf("GetObject() error: %v", err)
	}

	if string(data) != "image-bytes" {
		t.Errorf("data = %q, want image-bytes", data)
	}

	if !strings.HasPrefix(url, "https://neo.example.com/i/") {
		t.Errorf("url = %q, want https://neo.example.com/i/ prefix", url)
	}
}

func TestUploadKeyShape(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		nsfw        bool
		want        *regexp.Regexp
	}{
		{
			name:        "png",
			contentType: "image/png",
			want:        regexp.MustCompile(`^i/\d{4}-\d{2}/[0-9a-f-]{36}\.png$`),
		},
		{
			name:        "jpeg",
			contentType: "image/jpeg",
			want:        regexp.MustCompile(`^i/\d{4}-\d{2}/[0-9a-f-]{36}\.jpg$`),
		},
		{
			name:        "jxl",
			contentType: "image/jxl",
			want:        regexp.MustCompile(`^i/\d{4}-\d{2}/[0-9a-f-]{36}\.jxl$`),
		},
		{
			name:        "webp",
			contentType: "image/webp",
			want:        regexp.MustCompile(`^i/\d{4}-\d{2}/[0-9a-f-]{36}\.webp$`),
		},
		{
			name:        "nsfw",
			contentType: "image/png",
			nsfw:        true,
			want:        regexp.MustCompile(`^i/nsfw/\d{4}-\d{2}/[0-9a-f-]{36}\.png$`),
		},
		{
			name:        "unknown media type falls back to png",
			contentType: "application/octet-stream",
			want:        regexp.MustCompile(`^i/\d{4}-\d{2}/[0-9a-f-]{36}\.png$`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(t)

			_, key := upload(t, client, tt.contentType, tt.nsfw)

			if !tt.want.MatchString(key) {
				t.Errorf("key = %q, want a match for %s", key, tt.want)
			}
		})
	}
}

func TestUploadKeysAreUnique(t *testing.T) {
	client, _ := newTestClient(t)

	_, first := upload(t, client, "image/png", false)
	_, second := upload(t, client, "image/png", false)

	if first == second {
		t.Errorf("keys = %q and %q, want distinct keys", first, second)
	}
}

func TestSafePath(t *testing.T) {
	client, _ := newTestClient(t)

	tests := []struct {
		key     string
		wantErr bool
	}{
		{key: "i/2026-09/x.png"},
		{key: "i/nsfw/2026-09/x.png"},
		{key: "../../etc/passwd", wantErr: true},
		{key: "/etc/passwd", wantErr: true},
		{key: "x/y", wantErr: true},
		{key: "i/../../x", wantErr: true},
		{key: "a/avatars/x.png", wantErr: true},
		{key: "i//x.png", wantErr: true},
		{key: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			path, err := client.safePath(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("safePath(%q) error = nil, want an error", tt.key)
				}

				return
			}

			if err != nil {
				t.Fatalf("safePath(%q) error: %v", tt.key, err)
			}

			if !strings.HasPrefix(path, client.cfg.Dir) {
				t.Errorf("path = %q, want it under %q", path, client.cfg.Dir)
			}
		})
	}
}

func TestGetObjectRejectsTraversal(t *testing.T) {
	client, _ := newTestClient(t)

	if _, err := client.GetObject(context.Background(), "../../etc/passwd"); err == nil {
		t.Fatal("GetObject() error = nil, want a rejection")
	}
}

func TestUploadReportsWriteFailure(t *testing.T) {
	client, dir := newTestClient(t)

	if err := os.WriteFile(filepath.Join(dir, "i"), []byte("blocker"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	if _, err := client.UploadFile(context.Background(), []byte("bytes"), "image/png", false); err == nil {
		t.Fatal("UploadFile() error = nil, want a write failure")
	}
}

func TestUploadHonoursCancelledContext(t *testing.T) {
	client, _ := newTestClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.UploadFile(ctx, []byte("bytes"), "image/png", false); !errors.Is(err, context.Canceled) {
		t.Fatalf("UploadFile() error = %v, want context.Canceled", err)
	}
}
