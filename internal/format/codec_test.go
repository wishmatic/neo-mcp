package format

import (
	"strings"
	"testing"
)

func TestDecodeRoundTripsEveryFormat(t *testing.T) {
	source := transparentPNG(t)

	for _, format := range []Format{PNG, JPEG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			encoded, err := Convert(source, format)
			if err != nil {
				t.Fatalf("Convert() error: %v", err)
			}

			img, err := Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() error: %v", err)
			}

			if bounds := img.Bounds(); bounds.Dx() != 8 || bounds.Dy() != 8 {
				t.Errorf("bounds = %v, want 8x8", bounds)
			}
		})
	}
}

func TestDecodeRejectsNonImage(t *testing.T) {
	if _, err := Decode([]byte("not an image")); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestDimensionsReportsEveryFormat(t *testing.T) {
	source := solidPNG(t, 12, 7)

	for _, format := range []Format{PNG, JPEG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			encoded, err := Convert(source, format)
			if err != nil {
				t.Fatalf("Convert() error: %v", err)
			}

			width, height, err := Dimensions(encoded)
			if err != nil {
				t.Fatalf("Dimensions() error: %v", err)
			}

			if width != 12 || height != 7 {
				t.Errorf("Dimensions() = %dx%d, want 12x7", width, height)
			}
		})
	}
}

func TestDimensionsRejectsNonImage(t *testing.T) {
	if _, _, err := Dimensions([]byte("not an image")); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestEncodeProducesTargetFormat(t *testing.T) {
	img, err := Decode(transparentPNG(t))
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	for _, format := range []Format{PNG, JPEG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			got, err := Encode(img, format)
			if err != nil {
				t.Fatalf("Encode() error: %v", err)
			}

			if _, decoded := decodeImage(t, got); decoded != format.String() {
				t.Errorf("decoded format = %q, want %q", decoded, format)
			}
		})
	}
}

func TestEncodePreservesAlpha(t *testing.T) {
	img, err := Decode(transparentPNG(t))
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	for _, format := range []Format{PNG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			got, err := Encode(img, format)
			if err != nil {
				t.Fatalf("Encode() error: %v", err)
			}

			decoded, _ := decodeImage(t, got)

			if _, _, _, alpha := decoded.At(0, 0).RGBA(); alpha>>8 != 0 {
				t.Errorf("alpha = %d, want 0", alpha>>8)
			}

			if _, _, _, alpha := decoded.At(0, 4).RGBA(); alpha>>8 != 255 {
				t.Errorf("opaque alpha = %d, want 255", alpha>>8)
			}
		})
	}
}

func TestEncodeJPEGFlattensOntoWhite(t *testing.T) {
	img, err := Decode(halvedPNG(t))
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	got, err := Encode(img, JPEG)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	decoded, _ := decodeImage(t, got)

	r, g, b, alpha := decoded.At(2, 2).RGBA()
	if alpha>>8 != 255 {
		t.Errorf("alpha = %d, want an opaque 255", alpha>>8)
	}

	for name, channel := range map[string]uint32{"red": r >> 8, "green": g >> 8, "blue": b >> 8} {
		if channel < 220 {
			t.Errorf("transparent area %s = %d, want near white", name, channel)
		}
	}
}

func TestEncodeUnknownFormatErrors(t *testing.T) {
	img, err := Decode(transparentPNG(t))
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}

	if _, err := Encode(img, Format("gif")); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Fatalf("error = %v, want an format: prefix", err)
	}
}

func TestConvertProducesTargetFormat(t *testing.T) {
	source := transparentPNG(t)

	for _, format := range []Format{PNG, JPEG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			got, err := Convert(source, format)
			if err != nil {
				t.Fatalf("Convert() error: %v", err)
			}

			img, decoded := decodeImage(t, got)

			if decoded != format.String() {
				t.Errorf("decoded format = %q, want %q", decoded, format)
			}

			if bounds := img.Bounds(); bounds.Dx() != 8 || bounds.Dy() != 8 {
				t.Errorf("bounds = %v, want 8x8", bounds)
			}
		})
	}
}

func TestConvertPreservesAlpha(t *testing.T) {
	source := transparentPNG(t)

	for _, format := range []Format{PNG, JXL, WebP} {
		t.Run(format.String(), func(t *testing.T) {
			got, err := Convert(source, format)
			if err != nil {
				t.Fatalf("Convert() error: %v", err)
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

func TestConvertToJPEGFlattensOntoWhite(t *testing.T) {
	got, err := Convert(halvedPNG(t), JPEG)
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
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

func TestConvertPassthrough(t *testing.T) {
	pngSource := transparentPNG(t)

	got, err := Convert(pngSource, PNG)
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}

	if &got[0] != &pngSource[0] {
		t.Error("Convert() to the source format copied the bytes, want the input untouched")
	}

	webpSource, err := Convert(pngSource, WebP)
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}

	got, err = Convert(webpSource, WebP)
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}

	if &got[0] != &webpSource[0] {
		t.Error("Convert() to the source format copied the bytes, want the input untouched")
	}
}

func TestConvertErrors(t *testing.T) {
	if _, err := Convert([]byte("not an image"), WebP); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Errorf("error = %v, want an format: prefix", err)
	}

	if _, err := Convert(transparentPNG(t), Format("gif")); err == nil || !strings.HasPrefix(err.Error(), "format:") {
		t.Errorf("error = %v, want an format: prefix", err)
	}
}
