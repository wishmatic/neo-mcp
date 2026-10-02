package present

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStoredImagesPairsURLsWithImages(t *testing.T) {
	content, failures := StoredImages(
		[][]byte{testPNG(t)},
		[]string{"https://cdn.example.com/i/1.png"},
		InlineBudget{},
	)
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
	content, _ := StoredImages(
		[][]byte{testPNG(t)},
		[]string{"https://cdn.example.com/i/1.png"},
		InlineBudget{},
	)

	img, ok := content[1].(*mcp.ImageContent)
	if !ok {
		t.Fatalf("content[1] = %#v, want an image block", content[1])
	}

	if img.Annotations == nil || !slices.Equal(img.Annotations.Audience, []mcp.Role{RoleUser, RoleAssistant}) {
		t.Errorf("audience = %+v, want [user assistant]", img.Annotations)
	}
}

func TestStoredImagesShrinksToTheBudget(t *testing.T) {
	tests := []struct {
		name     string
		budget   InlineBudget
		wantEdge int
	}{
		{name: "default budget", budget: InlineBudget{}, wantEdge: DefaultInlineMaxEdge},
		{name: "named budget", budget: InlineBudget{MaxEdge: 64}, wantEdge: 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, failures := StoredImages(
				[][]byte{sizedPNG(t, 2048, 1024)},
				[]string{"https://cdn.example.com/i/1.png"},
				tt.budget,
			)
			if len(failures) != 0 {
				t.Fatalf("failures = %v, want none", failures)
			}

			img, ok := content[1].(*mcp.ImageContent)
			if !ok {
				t.Fatalf("content[1] = %#v, want an image block", content[1])
			}

			if len(img.Data) > DefaultInlineMaxBytes {
				t.Errorf("attached size = %d bytes, want at most %d", len(img.Data), DefaultInlineMaxBytes)
			}

			decoded, _, err := image.Decode(bytes.NewReader(img.Data))
			if err != nil {
				t.Fatalf("decode attached image: %v", err)
			}

			if bounds := decoded.Bounds(); bounds.Dx() != tt.wantEdge {
				t.Errorf("attached width = %d, want the longest edge shrunk to %d", bounds.Dx(), tt.wantEdge)
			}
		})
	}
}

func TestStoredImagesReportsAnUnreachableByteBudget(t *testing.T) {
	content, failures := StoredImages(
		[][]byte{sizedPNG(t, 64, 64)},
		[]string{"https://cdn.example.com/i/1.png"},
		InlineBudget{MaxEdge: 64, MaxBytes: 1},
	)

	if len(failures) != 1 {
		t.Fatalf("failures = %+v, want the byte budget to be unreachable", failures)
	}

	note, ok := content[1].(*mcp.TextContent)
	if !ok || !strings.Contains(note.Text, "could not be attached inline") {
		t.Fatalf("content[1] = %#v, want a failure note", content[1])
	}
}

func TestStoredImagesFailsSoft(t *testing.T) {
	content, failures := StoredImages(
		[][]byte{[]byte("not an image"), testPNG(t)},
		[]string{"https://cdn.example.com/i/1.png", "https://cdn.example.com/i/2.png"},
		InlineBudget{},
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
	content, failures := StoredImages(nil, []string{"https://cdn.example.com/i/1.png"}, InlineBudget{})
	if len(content) != 1 || len(failures) != 0 {
		t.Fatalf("content = %d, failures = %v, want the URL alone", len(content), failures)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()

	return sizedPNG(t, 64, 64)
}

func sizedPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 180, G: 40, B: 10, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}

	return buf.Bytes()
}
