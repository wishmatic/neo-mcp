package crop

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

type Options struct {
	// Threshold is the minimum alpha (0-255) for a pixel to count as content. Zero counts any visible pixel.
	Threshold uint8

	IsSquare bool

	// IsCircle cuts the square output into the circle inscribed in it. It implies IsSquare.
	IsCircle bool

	// Padding is the number of transparent pixels added on each side of the content.
	Padding int
}

// Apply trims src to the bounding box of its visible content and renders the requested padding, square, and circle.
func Apply(src image.Image, opts Options) (*image.RGBA, error) {
	if opts.Padding < 0 {
		return nil, fmt.Errorf("crop: padding must not be negative, got %d", opts.Padding)
	}

	content, err := contentBounds(src, opts.Threshold)
	if err != nil {
		return nil, err
	}

	return render(src, content, opts), nil
}

func ToContent(pngData []byte, opts Options) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("crop: decode image: %w", err)
	}

	out, err := Apply(src, opts)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("crop: encode image: %w", err)
	}

	return buf.Bytes(), nil
}

func contentBounds(src image.Image, threshold uint8) (image.Rectangle, error) {
	bounds := src.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X-1, bounds.Min.Y-1

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := src.At(x, y).RGBA()
			if alpha>>8 <= uint32(threshold) {
				continue
			}

			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x)
			maxY = max(maxY, y)
		}
	}

	if maxX < minX || maxY < minY {
		return image.Rectangle{}, fmt.Errorf("crop: image has no visible content")
	}

	return image.Rect(minX, minY, maxX+1, maxY+1), nil
}

func render(src image.Image, content image.Rectangle, opts Options) *image.RGBA {
	width, height := content.Dx(), content.Dy()
	size := image.Pt(width+2*opts.Padding, height+2*opts.Padding)
	offset := image.Pt(opts.Padding, opts.Padding)

	if opts.IsSquare {
		side := max(width, height) + 2*opts.Padding
		size = image.Pt(side, side)
		offset = image.Pt((side-width)/2, (side-height)/2)
	}

	dst := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	drawContent(dst, src, content, offset)

	if opts.IsCircle {
		applyCircle(dst)
	}

	return dst
}

func drawContent(dst *image.RGBA, src image.Image, content image.Rectangle, at image.Point) {
	target := image.Rectangle{Min: at, Max: at.Add(content.Size())}
	draw.Draw(dst, target, src, content.Min, draw.Src)
}
