package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestInlineDownscalesToMaxEdge(t *testing.T) {
	got, err := Inline(solidPNG(t, 2048, 1024), 1024, InlineMaxBytes)
	if err != nil {
		t.Fatalf("Inline() error: %v", err)
	}

	if got.MediaType != "image/webp" {
		t.Errorf("media type = %q, want image/webp", got.MediaType)
	}

	img, decoded := decodeImage(t, got.Data)

	if decoded != "webp" {
		t.Errorf("decoded format = %q, want webp", decoded)
	}

	if bounds := img.Bounds(); bounds.Dx() != 1024 || bounds.Dy() != 512 {
		t.Errorf("bounds = %v, want 1024x512", bounds)
	}
}

func TestInlineDoesNotUpscale(t *testing.T) {
	got, err := Inline(solidPNG(t, 8, 8), InlineMaxEdge, InlineMaxBytes)
	if err != nil {
		t.Fatalf("Inline() error: %v", err)
	}

	img, _ := decodeImage(t, got.Data)

	if bounds := img.Bounds(); bounds.Dx() != 8 || bounds.Dy() != 8 {
		t.Errorf("bounds = %v, want 8x8", bounds)
	}
}

func TestInlineMediaTypeIsAlwaysWebP(t *testing.T) {
	source := solidPNG(t, 32, 32)

	sources := map[string][]byte{"png": source}

	for _, format := range []Format{JPEG, JXL, WebP} {
		converted, err := Convert(source, format)
		if err != nil {
			t.Fatalf("Convert(%s) error: %v", format, err)
		}

		sources[format.String()] = converted
	}

	for name, data := range sources {
		t.Run(name, func(t *testing.T) {
			got, err := Inline(data, InlineMaxEdge, InlineMaxBytes)
			if err != nil {
				t.Fatalf("Inline() error: %v", err)
			}

			if got.MediaType != "image/webp" {
				t.Errorf("media type = %q, want image/webp", got.MediaType)
			}

			if !bytes.HasPrefix(got.Data, []byte("RIFF")) {
				t.Errorf("payload %q is not a WebP container", got.Data[:min(4, len(got.Data))])
			}

			if _, decoded := decodeImage(t, got.Data); decoded != "webp" {
				t.Errorf("decoded format = %q, want webp", decoded)
			}
		})
	}
}

func TestInlineRespectsByteBudget(t *testing.T) {
	const budget = 1500

	got, err := Inline(noisePNG(t, 256, 256), InlineMaxEdge, budget)
	if err != nil {
		t.Fatalf("Inline() error: %v", err)
	}

	if len(got.Data) > budget {
		t.Errorf("encoded size = %d, want at most %d", len(got.Data), budget)
	}

	decodeImage(t, got.Data)
}

func TestInlineImpossibleBudgetErrors(t *testing.T) {
	_, err := Inline(solidPNG(t, 16, 16), InlineMaxEdge, 1)
	if err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestInlineRejectsNonImage(t *testing.T) {
	_, err := Inline([]byte("not an image"), InlineMaxEdge, InlineMaxBytes)
	if err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestInlinePreservesTransparency(t *testing.T) {
	got, err := Inline(halfTransparentPNG(t, 128), 64, InlineMaxBytes)
	if err != nil {
		t.Fatalf("Inline() error: %v", err)
	}

	img, _ := decodeImage(t, got.Data)

	if bounds := img.Bounds(); bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Fatalf("bounds = %v, want 64x64", bounds)
	}

	if _, _, _, alpha := img.At(1, 1).RGBA(); alpha>>8 > 8 {
		t.Errorf("transparent alpha = %d, want near 0", alpha>>8)
	}

	if _, _, _, alpha := img.At(62, 1).RGBA(); alpha>>8 < 247 {
		t.Errorf("opaque alpha = %d, want near 255", alpha>>8)
	}
}
