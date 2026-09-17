package imgfmt

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func transparentPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			alpha := uint8(255)
			if y == 0 {
				alpha = 0
			}

			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 20), G: 90, B: 200, A: alpha})
		}
	}

	return encodePNG(t, img)
}

func halvedPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x < 8 && y < 8 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 0})

				continue
			}

			img.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
		}
	}

	return encodePNG(t, img)
}

func decodeImage(t *testing.T, data []byte) (image.Image, string) {
	t.Helper()

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}

	return img, format
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
	if _, err := Convert([]byte("not an image"), WebP); err == nil || !strings.HasPrefix(err.Error(), "imgfmt:") {
		t.Errorf("error = %v, want an imgfmt: prefix", err)
	}

	if _, err := Convert(transparentPNG(t), Format("gif")); err == nil || !strings.HasPrefix(err.Error(), "imgfmt:") {
		t.Errorf("error = %v, want an imgfmt: prefix", err)
	}
}
