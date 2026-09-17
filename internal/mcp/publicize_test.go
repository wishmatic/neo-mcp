package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/shortener"
)

var (
	publicizeJPEG = []byte("\xff\xd8\xff\xe0 pretend jpeg bytes")
	publicizePNG  = []byte("\x89PNG\r\n\x1a\n pretend png bytes")
)

type uploadCapture struct {
	path        string
	contentType string
}

func newPublicizeUploader(t *testing.T, captured *[]uploadCapture) *s3upload.Client {
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
	}, zapNop())
	if err != nil {
		t.Fatalf("s3upload.New() error: %v", err)
	}

	return uploader
}

func newPublicizeResolver(t *testing.T) *resolve.Resolver {
	t.Helper()

	resolver, err := resolve.New(nil, "")
	if err != nil {
		t.Fatalf("resolve.New() error: %v", err)
	}

	return resolver
}

func newPublicizeImageServer(t *testing.T, data []byte) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))

	t.Cleanup(server.Close)

	return server
}

func newPublicizeShortener(t *testing.T, status int, body string) *shortener.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return shortener.New(server.URL, "key", 0)
}

func TestPublicizeUploadsWithMediaType(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, publicizeJPEG)

	h := &handlers{
		log:      zapNop(),
		resolver: newPublicizeResolver(t),
		uploader: newPublicizeUploader(t, captured),
	}

	result, out, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: imageServer.URL + "/x.jpg"})
	if err != nil {
		t.Fatalf("publicize() error: %v", err)
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

	if !strings.HasPrefix(out.URL, "https://cdn.example.com/i/public/") || !strings.HasSuffix(out.URL, ".jpg") {
		t.Errorf("URL = %q, want https://cdn.example.com/i/public/...jpg", out.URL)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != out.URL {
		t.Errorf("content = %#v, want the public URL", result.Content[0])
	}
}

func TestPublicizePNG(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, publicizePNG)

	h := &handlers{
		log:      zapNop(),
		resolver: newPublicizeResolver(t),
		uploader: newPublicizeUploader(t, captured),
	}

	if _, _, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: imageServer.URL + "/x.png"}); err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	upload := (*captured)[0]
	if !strings.HasSuffix(upload.path, ".png") || upload.contentType != "image/png" {
		t.Errorf("upload = %+v, want .png and image/png", upload)
	}
}

func TestPublicizeShortensURL(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, publicizePNG)

	h := &handlers{
		log:       zapNop(),
		resolver:  newPublicizeResolver(t),
		uploader:  newPublicizeUploader(t, captured),
		shortener: newPublicizeShortener(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
	}

	_, out, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: imageServer.URL + "/x.png"})
	if err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	if out.URL != "https://short.example/abc" {
		t.Errorf("URL = %q, want https://short.example/abc", out.URL)
	}
}

func TestPublicizeShortenerFailureFallsBack(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, publicizePNG)

	h := &handlers{
		log:       zapNop(),
		resolver:  newPublicizeResolver(t),
		uploader:  newPublicizeUploader(t, captured),
		shortener: newPublicizeShortener(t, http.StatusInternalServerError, "boom"),
	}

	_, out, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: imageServer.URL + "/x.png"})
	if err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	if !strings.HasPrefix(out.URL, "https://cdn.example.com/i/public/") {
		t.Errorf("URL = %q, want the original public URL", out.URL)
	}
}

func TestPublicizeWithoutUploader(t *testing.T) {
	h := &handlers{log: zapNop(), resolver: newPublicizeResolver(t)}

	_, _, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: "https://example.com/x.png"})
	if err == nil || !strings.Contains(err.Error(), "S3 upload is not configured") {
		t.Fatalf("error = %v, want S3 upload is not configured", err)
	}
}

func TestPublicizeResolveError(t *testing.T) {
	captured := &[]uploadCapture{}

	h := &handlers{
		log:      zapNop(),
		resolver: newPublicizeResolver(t),
		uploader: newPublicizeUploader(t, captured),
	}

	_, _, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: "%%%not-an-image%%%"})
	if err == nil || !strings.HasPrefix(err.Error(), "publicize: fetch image:") {
		t.Fatalf("error = %v, want publicize: fetch image: prefix", err)
	}
}

func TestPublicizeRegistration(t *testing.T) {
	captured := &[]uploadCapture{}

	withUploader, err := New(zapNop(), nil, nil, newPublicizeUploader(t, captured), nil, newPublicizeResolver(t), nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	tools, err := connectSession(t, withUploader).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	var tool *mcp.Tool
	for _, candidate := range tools.Tools {
		if candidate.Name == "publicize" {
			tool = candidate
		}
	}

	if tool == nil {
		t.Fatalf("tools = %v, want publicize", toolNames(t, withUploader))
	}

	for _, want := range []string{"anyone", "explicit"} {
		if !strings.Contains(tool.Description, want) {
			t.Errorf("description %q does not mention %q", tool.Description, want)
		}
	}

	withoutUploader, err := New(zapNop(), nil, nil, nil, nil, newPublicizeResolver(t), nil)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withoutUploader); slices.Contains(names, "publicize") {
		t.Errorf("tools = %v, want no publicize", names)
	}
}

func TestPublicizeSchema(t *testing.T) {
	s := publicizeSchema()

	if !slices.Contains(s.Required, "image_url") {
		t.Error("image_url is not required")
	}

	if s.Properties["image_url"].Default != nil {
		t.Error("image_url must not have a default")
	}

	if len(s.Properties) != 1 {
		t.Errorf("properties = %v, want only image_url", s.Properties)
	}
}
