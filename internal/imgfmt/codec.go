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

// Encode encodes img in format, with the same behaviour Convert has: alpha is kept for every format except JPEG, which
// composites onto white.
func Encode(img image.Image, format Format) ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, img, format); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
