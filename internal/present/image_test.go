package present

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
)

func TestStoredImagesPairsURLsWithImages(t *testing.T) {
	content, failures := StoredImages([][]byte{testPNG(t)}, []string{"https://cdn.example.com/i/1.png"}, false)
	if len(failures) != 0 {
		t.Fatalf("failures = %v, want none", failures)
	}

	if len(content) != 2 {
		t.Fatalf("content = %d, want a URL and one image", len(content))
	}

	text, ok := content[0].(*mcp.TextContent)
	if !ok || text.Text != "https://cdn.example.com/i/1.png" {
		t.Fatalf("content[0] = %#v, want the URL as text", content[0])
	}

	img, ok := content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", content[1])
	}

	if img.MIMEType != "image/webp" {
		t.Errorf("mime type = %q, want image/webp", img.MIMEType)
	}

	if _, format, err := image.Decode(bytes.NewReader(img.Data)); err != nil || format != "webp" {
		t.Errorf("decode image = %q, %v, want webp", format, err)
	}
}

func TestStoredImagesAudience(t *testing.T) {
	tests := map[bool][]mcp.Role{
		false: {RoleUser},
		true:  {RoleAssistant, RoleUser},
	}

	for forAssistant, want := range tests {
		content, _ := StoredImages([][]byte{testPNG(t)}, []string{"https://cdn.example.com/i/1.png"}, forAssistant)

		img, ok := content[1].(*mcp.ImageContent)
		if !ok {
			t.Fatalf("content[1] = %#v, want an image block", content[1])
		}

		if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, want) {
			t.Errorf("StoredImages(forAssistant=%v) audience = %v, want %v", forAssistant, img.Annotations, want)
		}
	}
}

func TestStoredImagesFailsSoft(t *testing.T) {
	content, failures := StoredImages(
		[][]byte{[]byte("not an image"), testPNG(t)},
		[]string{"https://cdn.example.com/i/1.png", "https://cdn.example.com/i/2.png"},
		false,
	)

	if len(failures) != 1 || failures[0].Index != 1 {
		t.Fatalf("failures = %+v, want one at index 1", failures)
	}

	if len(content) != 4 {
		t.Fatalf("content = %d, want both URLs with a note and an image", len(content))
	}

	note, ok := content[1].(*mcp.TextContent)
	if !ok || !strings.Contains(note.Text, "could not be attached inline") {
		t.Fatalf("content[1] = %#v, want a failure note", content[1])
	}

	if _, ok := content[3].(*mcp.ImageContent); !ok {
		t.Fatalf("content[3] = %#v, want the second image", content[3])
	}
}

func TestStoredImagesWithoutImageData(t *testing.T) {
	content, failures := StoredImages(nil, []string{"https://cdn.example.com/i/1.png"}, false)
	if len(content) != 1 || len(failures) != 0 {
		t.Fatalf("content = %d, failures = %v, want the URL alone", len(content), failures)
	}
}

func TestInlineImage(t *testing.T) {
	image, err := imgfmt.Inline(testPNG(t), imgfmt.InlineMaxEdge, imgfmt.InlineMaxBytes)
	if err != nil {
		t.Fatalf("imgfmt.Inline() error: %v", err)
	}

	content := InlineImage("https://cdn.example.com/i/1.png", image)
	if len(content) != 2 {
		t.Fatalf("content = %d, want a caption and one image", len(content))
	}

	caption, ok := content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(caption.Text, "https://cdn.example.com/i/1.png") {
		t.Fatalf("content[0] = %#v, want a caption naming the URL", content[0])
	}

	block, ok := content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", content[1])
	}

	if block.Annotations == nil || !slices.Equal(block.Annotations.Audience, []mcp.Role{RoleAssistant, RoleUser}) {
		t.Errorf("audience = %+v, want [assistant user]", block.Annotations)
	}
}

func TestInlineImageFailure(t *testing.T) {
	content := InlineImageFailure("https://cdn.example.com/i/1.png", errors.New("boom"))
	if len(content) != 1 {
		t.Fatalf("content = %d, want a single note", len(content))
	}

	note, ok := content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(note.Text, "Could not inline") {
		t.Fatalf("content[0] = %#v, want a failure note", content[0])
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))

	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 180, G: 40, B: 10, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}

	return buf.Bytes()
}
