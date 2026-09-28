package format

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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

func solidPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 30, G: 120, B: 200, A: 255})
		}
	}

	return encodePNG(t, img)
}

func noisePNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	state := uint32(1)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			state = state*1664525 + 1013904223
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(state >> 24), G: uint8(state >> 16), B: uint8(state >> 8), A: 255})
		}
	}

	return encodePNG(t, img)
}

func halfTransparentPNG(t *testing.T, size int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			alpha := uint8(0)
			if x >= size/2 {
				alpha = 255
			}

			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 40, B: 10, A: alpha})
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

func mustDecode(t *testing.T, data []byte) image.Image {
	t.Helper()

	img, _ := decodeImage(t, data)

	return img
}
