package s3upload

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newTestUploader(t *testing.T, rt http.RoundTripper) *Client {
	t.Helper()

	cfg := Config{
		Endpoint:          "http://127.0.0.1:3900",
		PublicEndpoint:    "http://127.0.0.1:3900",
		Bucket:            "test-bucket",
		Region:            "test-region",
		AccessKey:         "write-key",
		SecretKey:         "write-secret",
		ReadonlyAccessKey: "read-key",
		ReadonlySecretKey: "read-secret",
	}

	httpClient := &http.Client{Transport: rt}

	return &Client{
		cfg: cfg,
		log: zap.NewNop(),
		writer: s3.New(s3.Options{
			Region:       cfg.Region,
			BaseEndpoint: ptr(cfg.Endpoint),
			UsePathStyle: true,
			HTTPClient:   httpClient,
			Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		}),
		reader: s3.New(s3.Options{
			Region:       cfg.Region,
			BaseEndpoint: ptr(cfg.PublicEndpoint),
			UsePathStyle: true,
			HTTPClient:   httpClient,
			Credentials:  credentials.NewStaticCredentialsProvider(cfg.ReadonlyAccessKey, cfg.ReadonlySecretKey, ""),
		}),
	}
}

func TestUploadImage(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   []byte
		gotCT     string
	)

	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotBody = b

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})

	u := newTestUploader(t, rt)

	want := []byte("pretend-png-bytes")
	url, err := u.UploadImage(context.Background(), want)
	if err != nil {
		t.Fatalf("UploadImage() error: %v", err)
	}

	// Only PutObject hits the network; presigning is computed locally.

	if gotMethod != http.MethodPut {
		t.Fatalf("request method = %q, want %q", gotMethod, http.MethodPut)
	}

	if !strings.HasPrefix(gotPath, "/test-bucket/") {
		t.Fatalf("request path = %q, want path-style prefix /test-bucket/", gotPath)
	}

	if !strings.HasSuffix(gotPath, ".png") {
		t.Fatalf("request path = %q, want .png suffix", gotPath)
	}

	if gotCT != "image/png" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "image/png")
	}

	if !bytes.Equal(gotBody, want) {
		t.Fatalf("request body = %q, want %q", gotBody, want)
	}

	if !strings.Contains(url, gotPath) {
		t.Fatalf("presigned URL %q does not contain object path %q", url, gotPath)
	}

	if !strings.Contains(url, "X-Amz-Signature=") {
		t.Fatalf("presigned URL %q missing X-Amz-Signature", url)
	}
}

func TestUploadImagePublicBaseURL(t *testing.T) {
	var gotPath string

	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})

	u := newTestUploader(t, rt)
	u.cfg.PublicBaseURL = "https://cdn.example.com"
	u.cfg.KeyPrefix = "i/mcp"

	url, err := u.UploadImage(context.Background(), []byte("pretend-png-bytes"))
	if err != nil {
		t.Fatalf("UploadImage() error: %v", err)
	}

	if !strings.HasPrefix(url, "https://cdn.example.com/i/mcp/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("URL = %q, want https://cdn.example.com/i/mcp/...png", url)
	}

	if strings.Contains(url, "?") {
		t.Fatalf("URL = %q, want no query string", url)
	}

	if !strings.HasPrefix(gotPath, "/test-bucket/i/mcp/") {
		t.Fatalf("request path = %q, want /test-bucket/i/mcp/ prefix", gotPath)
	}
}

func TestPublicObjectURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		key  string
		want string
	}{
		{
			name: "base without path",
			base: "https://cdn.example.com",
			key:  "i/mcp/2026-09/x.png",
			want: "https://cdn.example.com/i/mcp/2026-09/x.png",
		},
		{
			name: "base with trailing slash",
			base: "https://cdn.example.com/",
			key:  "i/mcp/x.png",
			want: "https://cdn.example.com/i/mcp/x.png",
		},
		{
			name: "base with path prefix",
			base: "https://cdn.example.com/cdn/",
			key:  "i/mcp/x.png",
			want: "https://cdn.example.com/cdn/i/mcp/x.png",
		},
		{
			name: "malformed base falls back to joining",
			base: "://bad",
			key:  "x.png",
			want: "://bad/x.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := publicObjectURL(tt.base, tt.key); got != tt.want {
				t.Fatalf("publicObjectURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRewriteEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		presigned string
		public    string
		want      string
	}{
		{
			name:      "swaps host",
			presigned: "http://127.0.0.1:3900/test-bucket/2026-08/x.png?X-Amz-Signature=abc&X-Amz-Expires=604800",
			public:    "https://garage.example.com",
			want:      "https://garage.example.com/test-bucket/2026-08/x.png?X-Amz-Signature=abc&X-Amz-Expires=604800",
		},
		{
			name:      "public endpoint with path prefix",
			presigned: "http://127.0.0.1:3900/test-bucket/2026-08/x.png?X-Amz-Signature=abc",
			public:    "https://cdn.example.com/images",
			want:      "https://cdn.example.com/images/test-bucket/2026-08/x.png?X-Amz-Signature=abc",
		},
		{
			name:      "invalid public endpoint returns original",
			presigned: "http://127.0.0.1:3900/test-bucket/x.png?X-Amz-Signature=abc",
			public:    "not a url",
			want:      "http://127.0.0.1:3900/test-bucket/x.png?X-Amz-Signature=abc",
		},
		{
			name:      "malformed presigned url returns original",
			presigned: "://bad",
			public:    "https://garage.example.com",
			want:      "://bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteEndpoint(tt.presigned, tt.public); got != tt.want {
				t.Fatalf("rewriteEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func ptr(s string) *string {
	return new(s)
}
