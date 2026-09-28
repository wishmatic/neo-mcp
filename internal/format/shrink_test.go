package format

import (
	"bytes"
	"strings"
	"testing"
)

const (
	testMaxEdge  = 1024
	testMaxBytes = 1 << 20
)

func TestShrinkDownscalesToMaxEdge(t *testing.T) {
	got, err := Shrink(solidPNG(t, 2048, 1024), 1024, testMaxBytes)
	if err != nil {
		t.Fatalf("Shrink() error: %v", err)
	}

	img, decoded := decodeImage(t, got)

	if decoded != "webp" {
		t.Errorf("decoded format = %q, want webp", decoded)
	}

	if bounds := img.Bounds(); bounds.Dx() != 1024 || bounds.Dy() != 512 {
		t.Errorf("bounds = %v, want 1024x512", bounds)
	}
}

func TestShrinkDoesNotUpscale(t *testing.T) {
	got, err := Shrink(solidPNG(t, 8, 8), testMaxEdge, testMaxBytes)
	if err != nil {
		t.Fatalf("Shrink() error: %v", err)
	}

	img, _ := decodeImage(t, got)

	if bounds := img.Bounds(); bounds.Dx() != 8 || bounds.Dy() != 8 {
		t.Errorf("bounds = %v, want 8x8", bounds)
	}
}

func TestShrinkAlwaysEncodesWebP(t *testing.T) {
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
			got, err := Shrink(data, testMaxEdge, testMaxBytes)
			if err != nil {
				t.Fatalf("Shrink() error: %v", err)
			}

			if !bytes.HasPrefix(got, []byte("RIFF")) {
				t.Errorf("payload %q is not a WebP container", got[:min(4, len(got))])
			}

			if _, decoded := decodeImage(t, got); decoded != "webp" {
				t.Errorf("decoded format = %q, want webp", decoded)
			}
		})
	}
}

func TestShrinkRespectsByteBudget(t *testing.T) {
	const budget = 1500

	got, err := Shrink(noisePNG(t, 256, 256), testMaxEdge, budget)
	if err != nil {
		t.Fatalf("Shrink() error: %v", err)
	}

	if len(got) > budget {
		t.Errorf("encoded size = %d, want at most %d", len(got), budget)
	}

	decodeImage(t, got)
}

func TestShrinkImpossibleBudgetErrors(t *testing.T) {
	_, err := Shrink(solidPNG(t, 16, 16), testMaxEdge, 1)
	if err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestShrinkRejectsNonImage(t *testing.T) {
	_, err := Shrink([]byte("not an image"), testMaxEdge, testMaxBytes)
	if err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestShrinkPreservesTransparency(t *testing.T) {
	got, err := Shrink(halfTransparentPNG(t, 128), 64, testMaxBytes)
	if err != nil {
		t.Fatalf("Shrink() error: %v", err)
	}

	img, _ := decodeImage(t, got)

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
