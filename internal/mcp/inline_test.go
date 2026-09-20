package mcp

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/present"
)

func newInlineSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	srv, err := New(Deps{Log: zapNop(), Resolver: newResolver(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return connectSession(t, srv)
}

func TestInlineCallTool(t *testing.T) {
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testImagePNG(t))
	}))
	t.Cleanup(images.Close)

	result, err := newInlineSession(t).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "inline",
		Arguments: map[string]any{"image_url": images.URL + "/x.png"},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	if len(result.Content) != 2 {
		t.Fatalf("content = %d, want a caption and one image", len(result.Content))
	}

	caption, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(caption.Text, images.URL) {
		t.Fatalf("content[0] = %#v, want a caption naming the URL", result.Content[0])
	}

	img, ok := result.Content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", result.Content[1])
	}

	if img.MIMEType != "image/webp" {
		t.Errorf("mime type = %q, want image/webp", img.MIMEType)
	}

	if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, []mcp.Role{present.RoleUser, present.RoleAssistant}) {
		t.Errorf("audience = %+v, want [user assistant]", img.Annotations)
	}

	if _, format, err := image.Decode(bytes.NewReader(img.Data)); err != nil || format != "webp" {
		t.Errorf("decode inline image = %q, %v, want webp", format, err)
	}
}

func TestInlineCallToolFailsSoftOnNonImage(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not an image"))
	}))
	t.Cleanup(plain.Close)

	result, err := newInlineSession(t).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "inline",
		Arguments: map[string]any{"image_url": plain.URL + "/x.txt"},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool() tool error: %+v", result.Content)
	}

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want a single failure note", len(result.Content))
	}

	note, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(note.Text, "Could not inline") {
		t.Fatalf("content[0] = %#v, want a failure note", result.Content[0])
	}
}

func TestInlineFetchFailure(t *testing.T) {
	h := &handlers{log: zapNop(), resolver: newResolver(t)}

	_, _, err := h.inline(context.Background(), nil, inlineInput{ImageURL: "http://127.0.0.1:1/x.png"})
	if err == nil || !strings.HasPrefix(err.Error(), "inline:") {
		t.Fatalf("error = %v, want an inline: prefix", err)
	}
}
