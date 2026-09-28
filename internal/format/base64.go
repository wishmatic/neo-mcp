package format

import (
	"encoding/base64"
	"fmt"
)

// DecodeRawBase64 decodes one base64 string, leaving the format untouched.
func DecodeRawBase64(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	return data, nil
}
