package publish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type uploadCapture struct {
	path        string
	contentType string
}

func newUploader(t *testing.T, captured *[]uploadCapture) *s3upload.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = append(*captured, uploadCapture{
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
		})

		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(server.Close)

	uploader, err := s3upload.New(s3upload.Config{
		Endpoint:      server.URL,
		Bucket:        "test-bucket",
		Region:        "test-region",
		AccessKey:     "write-key",
		SecretKey:     "write-secret",
		UsePathStyle:  true,
		PublicBaseURL: "https://cdn.example.com",
		KeyPrefix:     "i/images/user",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("s3upload.New() error: %v", err)
	}

	return uploader
}

func newShortener(t *testing.T, status int, body string) *shortener.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return shortener.New(server.URL, "key", 0)
}

func TestEnabled(t *testing.T) {
	if New(nil, nil, zap.NewNop()).Enabled() {
		t.Error("Enabled() = true without an uploader, want false")
	}

	if !New(newUploader(t, &[]uploadCapture{}), nil, zap.NewNop()).Enabled() {
		t.Error("Enabled() = false with an uploader, want true")
	}
}

func TestIsPublicURL(t *testing.T) {
	if New(nil, nil, zap.NewNop()).IsPublicURL("https://cdn.example.com/i/public/x.png") {
		t.Error("IsPublicURL() = true without an uploader, want false")
	}

	p := New(newUploader(t, &[]uploadCapture{}), nil, zap.NewNop())

	if !p.IsPublicURL("https://cdn.example.com/i/public/2026-09/x.png") {
		t.Error("IsPublicURL() = false for a public URL, want true")
	}

	if p.IsPublicURL("https://cdn.example.com/i/images/user/2026-09/x.png") {
		t.Error("IsPublicURL() = true for a private URL, want false")
	}
}

func TestImagesPublicNamespace(t *testing.T) {
	captured := &[]uploadCapture{}
	p := New(newUploader(t, captured), nil, zap.NewNop())

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("pretend-png-bytes")}, "image/png", true, false)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(*captured) != 1 || !strings.HasPrefix((*captured)[0].path, "/test-bucket/i/public/") {
		t.Fatalf("uploads = %+v, want one /test-bucket/i/public/ path", *captured)
	}

	if len(urls) != 1 || !strings.HasPrefix(urls[0], "https://cdn.example.com/i/public/") {
		t.Fatalf("urls = %v, want https://cdn.example.com/i/public/ prefix", urls)
	}
}

func TestImagesPrivateNamespace(t *testing.T) {
	captured := &[]uploadCapture{}
	p := New(newUploader(t, captured), nil, zap.NewNop())

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("pretend-png-bytes")}, "image/png", false, false)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(*captured) != 1 || !strings.HasPrefix((*captured)[0].path, "/test-bucket/i/images/user/") {
		t.Fatalf("uploads = %+v, want one /test-bucket/i/images/user/ path", *captured)
	}

	if len(urls) != 1 || !strings.HasPrefix(urls[0], "https://cdn.example.com/i/images/user/") {
		t.Fatalf("urls = %v, want https://cdn.example.com/i/images/user/ prefix", urls)
	}
}

func TestImagesNSFWNamespace(t *testing.T) {
	captured := &[]uploadCapture{}
	p := New(newUploader(t, captured), nil, zap.NewNop())

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("pretend-png-bytes")}, "image/png", false, true)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(*captured) != 1 || !strings.HasPrefix((*captured)[0].path, "/test-bucket/i/images/user/nsfw/") {
		t.Fatalf("uploads = %+v, want one /test-bucket/i/images/user/nsfw/ path", *captured)
	}

	if len(urls) != 1 || !strings.HasPrefix(urls[0], "https://cdn.example.com/i/images/user/nsfw/") {
		t.Fatalf("urls = %v, want https://cdn.example.com/i/images/user/nsfw/ prefix", urls)
	}
}

func TestImagesKeepsOrderAndCount(t *testing.T) {
	captured := &[]uploadCapture{}
	p := New(newUploader(t, captured), nil, zap.NewNop())

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a"), []byte("b"), []byte("c")}, "image/png", false, false)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(urls) != 3 || len(*captured) != 3 {
		t.Fatalf("urls = %v, uploads = %+v, want three of each", urls, *captured)
	}

	if urls[0] == urls[1] || urls[1] == urls[2] {
		t.Errorf("urls = %v, want distinct object keys", urls)
	}
}

func TestImagesUploadsWithContentType(t *testing.T) {
	tests := []struct {
		contentType string
		wantExt     string
	}{
		{contentType: "image/png", wantExt: ".png"},
		{contentType: "image/jpeg", wantExt: ".jpg"},
		{contentType: "image/jxl", wantExt: ".jxl"},
		{contentType: "image/webp", wantExt: ".webp"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			captured := &[]uploadCapture{}
			p := New(newUploader(t, captured), nil, zap.NewNop())

			images := [][]byte{[]byte("bytes")}

			urls, err := p.Images(context.Background(), "txt2img", images, tt.contentType, false, false)
			if err != nil {
				t.Fatalf("Images() error: %v", err)
			}

			if len(*captured) != 1 || (*captured)[0].contentType != tt.contentType {
				t.Fatalf("uploads = %+v, want content type %s", *captured, tt.contentType)
			}

			if !strings.HasSuffix((*captured)[0].path, tt.wantExt) || !strings.HasSuffix(urls[0], tt.wantExt) {
				t.Errorf("path = %q, url = %q, want %s suffix", (*captured)[0].path, urls[0], tt.wantExt)
			}
		})
	}
}

func TestImagesShortens(t *testing.T) {
	p := New(
		newUploader(t, &[]uploadCapture{}),
		newShortener(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
		zap.NewNop(),
	)

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a"), []byte("b")}, "image/png", false, false)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	for _, url := range urls {
		if url != "https://short.example/abc" {
			t.Errorf("url = %q, want the short URL", url)
		}
	}
}

func TestImagesShortenerFailureFallsBack(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	p := New(
		newUploader(t, &[]uploadCapture{}),
		newShortener(t, http.StatusInternalServerError, "boom"),
		zap.New(core),
	)

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a")}, "image/png", false, false)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(urls) != 1 || !strings.HasPrefix(urls[0], "https://cdn.example.com/") {
		t.Fatalf("urls = %v, want the original URL", urls)
	}

	if count := logs.FilterMessageSnippet("url shortening failed").Len(); count != 1 {
		t.Errorf("warnings = %d, want one shortening warning", count)
	}
}

func newFailingUploader(t *testing.T) *s3upload.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))

	t.Cleanup(server.Close)

	uploader, err := s3upload.New(s3upload.Config{
		Endpoint:      server.URL,
		Bucket:        "test-bucket",
		Region:        "test-region",
		AccessKey:     "write-key",
		SecretKey:     "write-secret",
		UsePathStyle:  true,
		PublicBaseURL: "https://cdn.example.com",
		KeyPrefix:     "i/images/user",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("s3upload.New() error: %v", err)
	}

	return uploader
}

func TestImagesUploadFailure(t *testing.T) {
	p := New(newFailingUploader(t), nil, zap.NewNop())

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a")}, "image/png", false, false)
	if err == nil {
		t.Fatal("Images() error = nil, want the upload failure")
	}

	if urls != nil {
		t.Errorf("urls = %v, want none on failure", urls)
	}
}

func TestFileUploadFailure(t *testing.T) {
	p := New(newFailingUploader(t), nil, zap.NewNop())

	url, err := p.File(context.Background(), "publicize", []byte("a"), "image/png", true)
	if err == nil {
		t.Fatal("File() error = nil, want the upload failure")
	}

	if url != "" {
		t.Errorf("url = %q, want empty on failure", url)
	}
}

func TestFileUploadsWithMediaType(t *testing.T) {
	captured := &[]uploadCapture{}
	p := New(newUploader(t, captured), nil, zap.NewNop())

	url, err := p.File(context.Background(), "publicize", []byte("jpeg-bytes"), "image/jpeg", true)
	if err != nil {
		t.Fatalf("File() error: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("uploads = %d, want 1", len(*captured))
	}

	upload := (*captured)[0]
	if !strings.HasPrefix(upload.path, "/test-bucket/i/public/") || !strings.HasSuffix(upload.path, ".jpg") {
		t.Errorf("upload path = %q, want /test-bucket/i/public/...jpg", upload.path)
	}

	if upload.contentType != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", upload.contentType)
	}

	if !strings.HasPrefix(url, "https://cdn.example.com/i/public/") || !strings.HasSuffix(url, ".jpg") {
		t.Errorf("url = %q, want https://cdn.example.com/i/public/...jpg", url)
	}
}

func TestFileShortensAndFallsBack(t *testing.T) {
	p := New(
		newUploader(t, &[]uploadCapture{}),
		newShortener(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
		zap.NewNop(),
	)

	url, err := p.File(context.Background(), "publicize", []byte("png"), "image/png", false)
	if err != nil {
		t.Fatalf("File() error: %v", err)
	}

	if url != "https://short.example/abc" {
		t.Errorf("url = %q, want the short URL", url)
	}

	failing := New(
		newUploader(t, &[]uploadCapture{}),
		newShortener(t, http.StatusInternalServerError, "boom"),
		zap.NewNop(),
	)

	url, err = failing.File(context.Background(), "publicize", []byte("png"), "image/png", false)
	if err != nil {
		t.Fatalf("File() error: %v", err)
	}

	if !strings.HasPrefix(url, "https://cdn.example.com/") {
		t.Errorf("url = %q, want the original URL", url)
	}
}
