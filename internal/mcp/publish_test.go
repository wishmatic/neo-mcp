package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
)

func newPublishUploader(t *testing.T) (*s3upload.Client, *[]string) {
	t.Helper()

	paths := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*paths = append(*paths, r.URL.Path)
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
	}, zapNop())
	if err != nil {
		t.Fatalf("s3upload.New() error: %v", err)
	}

	return uploader, paths
}

func TestPublishImagesPublicNamespace(t *testing.T) {
	uploader, paths := newPublishUploader(t)

	result, out, err := publishImages(
		context.Background(),
		zapNop(),
		"txt2img",
		[][]byte{[]byte("pretend-png-bytes")},
		true,
		uploader,
		nil,
	)
	if err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	if len(*paths) != 1 || !strings.HasPrefix((*paths)[0], "/test-bucket/i/public/") {
		t.Fatalf("request paths = %v, want one /test-bucket/i/public/ path", *paths)
	}

	if len(out.URLs) != 1 || !strings.HasPrefix(out.URLs[0], "https://cdn.example.com/i/public/") {
		t.Fatalf("URLs = %v, want https://cdn.example.com/i/public/ prefix", out.URLs)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != out.URLs[0] {
		t.Fatalf("content = %#v, want the public URL", result.Content[0])
	}
}

func TestPublishImagesPrivateNamespace(t *testing.T) {
	uploader, paths := newPublishUploader(t)

	_, out, err := publishImages(
		context.Background(),
		zapNop(),
		"txt2img",
		[][]byte{[]byte("pretend-png-bytes")},
		false,
		uploader,
		nil,
	)
	if err != nil {
		t.Fatalf("publishImages() error: %v", err)
	}

	if len(*paths) != 1 || !strings.HasPrefix((*paths)[0], "/test-bucket/i/images/user/") {
		t.Fatalf("request paths = %v, want one /test-bucket/i/images/user/ path", *paths)
	}

	if len(out.URLs) != 1 || !strings.HasPrefix(out.URLs[0], "https://cdn.example.com/i/images/user/") {
		t.Fatalf("URLs = %v, want https://cdn.example.com/i/images/user/ prefix", out.URLs)
	}
}

func TestPublishImagesWithoutUploader(t *testing.T) {
	for _, public := range []bool{false, true} {
		result, out, err := publishImages(
			context.Background(),
			zapNop(),
			"txt2img",
			[][]byte{[]byte("pretend-png-bytes")},
			public,
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("publishImages(public=%v) error: %v", public, err)
		}

		if out.Count != 1 || len(out.URLs) != 0 {
			t.Fatalf("output = %+v, want one image and no URLs", out)
		}

		if len(result.Content) != 1 {
			t.Fatalf("content = %d, want 1", len(result.Content))
		}

		image, ok := result.Content[0].(*mcp.ImageContent)
		if !ok || image.MIMEType != "image/png" {
			t.Fatalf("content = %#v, want inline image/png", result.Content[0])
		}
	}
}
