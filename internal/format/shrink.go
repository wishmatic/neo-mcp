package format

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/gen2brain/vpx/webp"
)

var shrinkQualities = []int{encodeQuality, 75, 60, 45, 30}

// Shrink re-encodes image data as WebP that fits within maxBytes.
//
// WebP is a relatively small format, so shrinking is used primarily so vision models can process
// images that are too large to fit in memory. Also note that shrinking is lossy.
func Shrink(data []byte, maxEdge, maxBytes int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("format: decode image to shrink: %w", err)
	}

	img = downscaleToEdge(img, maxEdge)

	for {
		encoded, err := encodeUnder(img, maxBytes)
		if err != nil {
			return nil, err
		}

		if encoded != nil {
			return encoded, nil
		}

		if bounds := img.Bounds(); bounds.Dx() <= 1 && bounds.Dy() <= 1 {
			return nil, fmt.Errorf("format: image cannot be encoded within %d bytes", maxBytes)
		}

		img = halve(img)
	}
}

func encodeUnder(img image.Image, maxBytes int) ([]byte, error) {
	for _, quality := range shrinkQualities {
		var buf bytes.Buffer
		if err := webp.Encode(&buf, img, webp.EncodeOptions{Quality: quality, Method: -1}); err != nil {
			return nil, fmt.Errorf("format: encode shrunk webp: %w", err)
		}

		if buf.Len() <= maxBytes {
			return buf.Bytes(), nil
		}
	}

	return nil, nil
}

func downscaleToEdge(img image.Image, maxEdge int) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := max(width, height)

	if longest <= maxEdge {
		return img
	}

	scale := float64(maxEdge) / float64(longest)

	return resample(img, max(1, int(math.Round(float64(width)*scale))), max(1, int(math.Round(float64(height)*scale))))
}

func halve(img image.Image) image.Image {
	bounds := img.Bounds()

	return resample(img, max(1, bounds.Dx()/2), max(1, bounds.Dy()/2))
}

func resample(img image.Image, width, height int) *image.NRGBA {
	src := img.Bounds()
	srcWidth, srcHeight := src.Dx(), src.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		y0 := src.Min.Y + y*srcHeight/height
		y1 := max(y0+1, src.Min.Y+(y+1)*srcHeight/height)

		for x := 0; x < width; x++ {
			x0 := src.Min.X + x*srcWidth/width
			x1 := max(x0+1, src.Min.X+(x+1)*srcWidth/width)

			dst.SetNRGBA(x, y, averagePixels(img, x0, y0, x1, y1))
		}
	}

	return dst
}

// averagePixels averages alpha-premultiplied samples and unpremultiplies, so transparent regions do not darken the
// edges they blend into.
func averagePixels(img image.Image, x0, y0, x1, y1 int) color.NRGBA {
	var r, g, b, a, n uint64

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			r += uint64(cr)
			g += uint64(cg)
			b += uint64(cb)
			a += uint64(ca)
			n++
		}
	}

	if n == 0 || a == 0 {
		return color.NRGBA{}
	}

	r, g, b, a = r/n, g/n, b/n, a/n

	return color.NRGBA{
		R: uint8((r * 0xffff / a) >> 8),
		G: uint8((g * 0xffff / a) >> 8),
		B: uint8((b * 0xffff / a) >> 8),
		A: uint8(a >> 8),
	}
}
