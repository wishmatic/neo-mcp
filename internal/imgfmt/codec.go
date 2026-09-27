package imgfmt

import (
	"bytes"
	"fmt"
	"image"
)

// Decode decodes image data in any format the codecs register, which is every format Convert can produce.
func Decode(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imgfmt: decode image: %w", err)
	}

	return img, nil
}

// Dimensions returns the pixel size of image data in any format the codecs register, reading only the header rather
// than the pixels.
func Dimensions(data []byte) (width, height int, err error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("imgfmt: decode image config: %w", err)
	}

	return config.Width, config.Height, nil
}

// Encode encodes img in format, with the same behaviour Convert has: alpha is kept for every format except JPEG, which
// composites onto white.
func Encode(img image.Image, format Format) ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, img, format); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
