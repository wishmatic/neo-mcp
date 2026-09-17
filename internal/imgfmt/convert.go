package imgfmt

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/gen2brain/jxl"
	"github.com/gen2brain/vpx/webp"
)

const encodeQuality = 90

// Method -1 selects the codec's default quality/speed trade-off; the zero value would pick the fastest, largest-output
// method instead.
var webpEncodeOptions = webp.EncodeOptions{
	Quality: encodeQuality,
	Method:  -1,
}

// Convert re-encodes image data to the requested format, returning the input untouched when it is already in that
// format.
func Convert(data []byte, format Format) ([]byte, error) {
	if format.MediaType() == "" {
		return nil, fmt.Errorf("imgfmt: cannot convert to unknown format %q", string(format))
	}

	_, source, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imgfmt: decode source image: %w", err)
	}

	if source == format.String() {
		return data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imgfmt: decode source image: %w", err)
	}

	var buf bytes.Buffer
	if err := encode(&buf, img, format); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func encode(w io.Writer, img image.Image, format Format) error {
	var err error

	switch format {
	case PNG:
		err = png.Encode(w, img)
	case JPEG:
		err = jpeg.Encode(w, flattenOntoWhite(img), &jpeg.Options{Quality: encodeQuality})
	case JXL:
		err = jxl.Encode(w, img, jxl.EncodeOptions{Quality: encodeQuality})
	case WebP:
		err = webp.Encode(w, img, webpEncodeOptions)
	default:
		return fmt.Errorf("imgfmt: cannot encode unknown format %q", string(format))
	}

	if err != nil {
		return fmt.Errorf("imgfmt: encode %s: %w", format, err)
	}

	return nil
}

func flattenOntoWhite(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)

	return dst
}
