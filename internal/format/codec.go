package format

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

func Encode(img image.Image, format Format) ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, img, format); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func Decode(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("format: decode image: %w", err)
	}

	return img, nil
}

func Convert(data []byte, format Format) ([]byte, error) {
	if format.MediaType() == "" {
		return nil, fmt.Errorf("format: cannot convert to unknown format %q", string(format))
	}

	_, source, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("format: decode source image: %w", err)
	}

	if source == format.String() {
		return data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("format: decode source image: %w", err)
	}

	var buf bytes.Buffer
	if err := encode(&buf, img, format); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func Dimensions(data []byte) (width, height int, err error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("format: decode image config: %w", err)
	}

	return config.Width, config.Height, nil
}

func encode(w io.Writer, img image.Image, format Format) error {
	var err error

	switch format {
	case PNG:
		err = png.Encode(w, img)
	case JPEG:
		// JPEG has no alpha channel, so we flatten the image onto a white background.

		err = jpeg.Encode(w, flattenOntoWhite(img), &jpeg.Options{Quality: encodeQuality})
	case JXL:
		err = jxl.Encode(w, img, jxl.EncodeOptions{Quality: encodeQuality})
	case WebP:
		err = webp.Encode(w, img, webpEncodeOptions)
	default:
		return fmt.Errorf("format: cannot encode unknown format %q", string(format))
	}

	if err != nil {
		return fmt.Errorf("format: encode %s: %w", format, err)
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
