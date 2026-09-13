package s3upload

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newTestUploader(t *testing.T, rt http.RoundTripper) *Uploader {
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
		PresignExpiry:     7 * 24 * time.Hour,
	}

	httpClient := &http.Client{Transport: rt}

	return &Uploader{
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

func TestNewMissingConfig(t *testing.T) {
	base := Config{
		Endpoint:          "http://127.0.0.1:3900",
		Bucket:            "test-bucket",
		Region:            "test-region",
		AccessKey:         "write-key",
		SecretKey:         "write-secret",
		ReadonlyAccessKey: "read-key",
		ReadonlySecretKey: "read-secret",
	}

	fields := map[string]func(*Config){
		"endpoint":            func(c *Config) { c.Endpoint = "" },
		"bucket":              func(c *Config) { c.Bucket = "" },
		"region":              func(c *Config) { c.Region = "" },
		"access key":          func(c *Config) { c.AccessKey = "" },
		"secret key":          func(c *Config) { c.SecretKey = "" },
		"readonly access key": func(c *Config) { c.ReadonlyAccessKey = "" },
		"readonly secret key": func(c *Config) { c.ReadonlySecretKey = "" },
	}

	for name, mutate := range fields {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)

			if _, err := New(cfg, zap.NewNop()); err == nil {
				t.Fatalf("New() with missing %s expected error, got nil", name)
			}
		})
	}
}

//go:fix inline
func ptr(s string) *string { return new(s) }
