package format

import (
	"strings"
	"testing"
)

func TestReformatProducesTargetFormat(t *testing.T) {
	for _, source := range []Format{PNG, JPEG, JXL, WebP} {
		sourceData, err := Encode(mustDecode(t, transparentPNG(t)), source)
		if err != nil {
			t.Fatalf("Encode(%s) error: %v", source, err)
		}

		for _, target := range []Format{PNG, JPEG, JXL, WebP} {
			if source == target {
				continue
			}

			t.Run(source.String()+" to "+target.String(), func(t *testing.T) {
				got, err := Reformat(sourceData, target)
				if err != nil {
					t.Fatalf("Reformat() error: %v", err)
				}

				img, decoded := decodeImage(t, got)

				if decoded != target.String() {
					t.Errorf("decoded format = %q, want %q", decoded, target)
				}

				if bounds := img.Bounds(); bounds.Dx() != 8 || bounds.Dy() != 8 {
					t.Errorf("bounds = %v, want 8x8", bounds)
				}
			})
		}
	}
}

func TestReformatRejectsSameFormat(t *testing.T) {
	for _, format := range []Format{PNG, JPEG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			source, err := Encode(mustDecode(t, transparentPNG(t)), format)
			if err != nil {
				t.Fatalf("Encode() error: %v", err)
			}

			got, err := Reformat(source, format)
			if err == nil {
				t.Fatalf("Reformat() error = nil, want a refusal to reconvert to %s", format)
			}

			if !strings.HasPrefix(err.Error(), "format:") || !strings.Contains(err.Error(), format.String()) {
				t.Errorf("error = %v, want an format: prefix naming %s", err, format)
			}

			if got != nil {
				t.Errorf("Reformat() = %d bytes, want no output", len(got))
			}
		})
	}
}

func TestReformatPreservesAlpha(t *testing.T) {
	pairs := []struct {
		source Format
		target Format
	}{
		{PNG, JXL},
		{PNG, WebP},
		{JXL, PNG},
	}

	for _, pair := range pairs {
		t.Run(pair.source.String()+" to "+pair.target.String(), func(t *testing.T) {
			source, err := Encode(mustDecode(t, transparentPNG(t)), pair.source)
			if err != nil {
				t.Fatalf("Encode() error: %v", err)
			}

			got, err := Reformat(source, pair.target)
			if err != nil {
				t.Fatalf("Reformat() error: %v", err)
			}

			img, _ := decodeImage(t, got)

			if _, _, _, alpha := img.At(0, 0).RGBA(); alpha>>8 != 0 {
				t.Errorf("alpha = %d, want 0", alpha>>8)
			}

			if _, _, _, alpha := img.At(0, 4).RGBA(); alpha>>8 != 255 {
				t.Errorf("opaque alpha = %d, want 255", alpha>>8)
			}
		})
	}
}

func TestReformatToJPEGFlattensOntoWhite(t *testing.T) {
	got, err := Reformat(halvedPNG(t), JPEG)
	if err != nil {
		t.Fatalf("Reformat() error: %v", err)
	}

	img, _ := decodeImage(t, got)

	r, g, b, alpha := img.At(2, 2).RGBA()
	if alpha>>8 != 255 {
		t.Errorf("alpha = %d, want an opaque 255", alpha>>8)
	}

	for name, channel := range map[string]uint32{"red": r >> 8, "green": g >> 8, "blue": b >> 8} {
		if channel < 220 {
			t.Errorf("transparent area %s = %d, want near white", name, channel)
		}
	}
}

func TestReformatErrors(t *testing.T) {
	if _, err := Reformat([]byte("not an image"), WebP); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Errorf("error = %v, want an format: prefix", err)
	}

	if _, err := Reformat(transparentPNG(t), Format("gif")); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Errorf("error = %v, want an format: prefix", err)
	}
}
