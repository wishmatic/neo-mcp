package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/publish"
	"github.com/wishmatic/neo-mcp/internal/resolve"
	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/shortener"
)

type uploadCapture struct {
	path        string
	contentType string
}

func newShortenClient(t *testing.T, status int, body string) *shortener.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return shortener.New(server.URL, "key", 0)
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
func TestPublicizeUploadsWithMediaType(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, testImageJPEG(t))

	h := &handlers{
		log:       zapNop(),
		resolver:  newPublicizeResolver(t),
		publisher: publish.New(newPublicizeUploader(t, captured), nil, zapNop()),
	}

	result, out, err := h.publicize(context.Background(), nil, publicizeInput{
		ImageURL: imageServer.URL + "/x.jpg",
		Format:   "jpeg",
	})
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
	imageServer := newPublicizeImageServer(t, testImagePNG(t))

	h := &handlers{
		log:       zapNop(),
		resolver:  newPublicizeResolver(t),
		publisher: publish.New(newPublicizeUploader(t, captured), nil, zapNop()),
	}

	call := publicizeInput{ImageURL: imageServer.URL + "/x.png", Format: "png"}

	if _, _, err := h.publicize(context.Background(), nil, call); err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	upload := (*captured)[0]
	if !strings.HasSuffix(upload.path, ".png") || upload.contentType != "image/png" {
		t.Errorf("upload = %+v, want .png and image/png", upload)
	}
}

func TestPublicizeUsesHandlerDefaultFormat(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, testImagePNG(t))

	h := &handlers{
		log:           zapNop(),
		resolver:      newPublicizeResolver(t),
		publisher:     publish.New(newPublicizeUploader(t, captured), nil, zapNop()),
		defaultFormat: imgfmt.JXL,
	}

	call := publicizeInput{ImageURL: imageServer.URL + "/x.png"}
	_, out, err := h.publicize(context.Background(), nil, call)
	if err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	upload := (*captured)[0]
	if !strings.HasSuffix(upload.path, ".jxl") || upload.contentType != "image/jxl" {
		t.Errorf("upload = %+v, want .jxl and image/jxl", upload)
	}

	if !strings.HasSuffix(out.URL, ".jxl") {
		t.Errorf("URL = %q, want a .jxl suffix", out.URL)
	}
}

func TestPublicizeShortensURL(t *testing.T) {
	captured := &[]uploadCapture{}
	imageServer := newPublicizeImageServer(t, testImagePNG(t))

	h := &handlers{
		log:      zapNop(),
		resolver: newPublicizeResolver(t),
		publisher: publish.New(
			newPublicizeUploader(t, captured),
			newShortenClient(t, http.StatusOK, `{"success":true,"shorturl":"https://short.example/abc"}`),
			zapNop(),
		),
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
	imageServer := newPublicizeImageServer(t, testImagePNG(t))

	h := &handlers{
		log:      zapNop(),
		resolver: newPublicizeResolver(t),
		publisher: publish.New(
			newPublicizeUploader(t, captured),
			newShortenClient(t, http.StatusInternalServerError, "boom"),
			zapNop(),
		),
	}

	_, out, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: imageServer.URL + "/x.png"})
	if err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	if !strings.HasPrefix(out.URL, "https://cdn.example.com/i/public/") {
		t.Errorf("URL = %q, want the original public URL", out.URL)
	}
}

func TestPublicizeAlreadyPublicSkipsWork(t *testing.T) {
	captured := &[]uploadCapture{}

	h := &handlers{
		log:       zapNop(),
		resolver:  newPublicizeResolver(t),
		publisher: publish.New(newPublicizeUploader(t, captured), nil, zapNop()),
	}

	const publicURL = "https://cdn.example.com/i/public/2026-09/x.png"

	result, out, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: publicURL})
	if err != nil {
		t.Fatalf("publicize() error: %v", err)
	}

	if len(*captured) != 0 {
		t.Errorf("uploads = %d, want none for an already public image", len(*captured))
	}

	if out.URL != publicURL {
		t.Errorf("URL = %q, want %q unchanged", out.URL, publicURL)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != publicURL {
		t.Errorf("content = %#v, want the unchanged public URL", result.Content[0])
	}
}

func TestPublicizeWithoutUploader(t *testing.T) {
	h := &handlers{log: zapNop(), resolver: newPublicizeResolver(t), publisher: publish.New(nil, nil, zapNop())}

	_, _, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: "https://example.com/x.png"})
	if err == nil || !strings.Contains(err.Error(), "S3 upload is not configured") {
		t.Fatalf("error = %v, want S3 upload is not configured", err)
	}
}

func TestPublicizeResolveError(t *testing.T) {
	captured := &[]uploadCapture{}

	h := &handlers{
		log:       zapNop(),
		resolver:  newPublicizeResolver(t),
		publisher: publish.New(newPublicizeUploader(t, captured), nil, zapNop()),
	}

	_, _, err := h.publicize(context.Background(), nil, publicizeInput{ImageURL: "%%%not-an-image%%%"})
	if err == nil || !strings.HasPrefix(err.Error(), "publicize: fetch image:") {
		t.Fatalf("error = %v, want publicize: fetch image: prefix", err)
	}
}

func TestPublicizeRegistration(t *testing.T) {
	captured := &[]uploadCapture{}

	withUploader, err := New(Deps{
		Log:       zapNop(),
		Publisher: publish.New(newPublicizeUploader(t, captured), nil, zapNop()),
		Resolver:  newPublicizeResolver(t),
	})
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

	withoutUploader, err := New(Deps{Log: zapNop(), Resolver: newPublicizeResolver(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if names := toolNames(t, withoutUploader); slices.Contains(names, "publicize") {
		t.Errorf("tools = %v, want no publicize", names)
	}
}

func TestPublicizeSchema(t *testing.T) {
	s := publicizeSchema(imgfmt.Default)

	if !slices.Contains(s.Required, "image_url") {
		t.Error("image_url is not required")
	}

	if s.Properties["image_url"].Default != nil {
		t.Error("image_url must not have a default")
	}

	if len(s.Properties) != 2 {
		t.Errorf("properties = %v, want image_url and format", s.Properties)
	}
}
