package format

import (
	"bytes"
	"fmt"
	"image"

	"github.com/gen2brain/vpx/webp"
)

const encodeQuality = 90

// Method -1 selects the codec's default quality/speed trade-off; the zero value would pick the fastest, largest-output
// method instead.
var webpEncodeOptions = webp.EncodeOptions{
	Quality: encodeQuality,
	Method:  -1,
}

// Reformat re-encodes image data to the requested format and fails when the data is already in that format, so a caller
// that asked for a conversion either gets new bytes or an error, never its input back.
func Reformat(data []byte, format Format) ([]byte, error) {
	_, source, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("format: decode source image: %w", err)
	}

	if source == format.String() {
		return nil, fmt.Errorf("format: source image is already %s", format)
	}

	return Convert(data, format)
}
