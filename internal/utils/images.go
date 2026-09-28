package utils

import (
	"encoding/base64"
	"fmt"
)

func Encode(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func Decode(encoded []string) ([][]byte, error) {
	decoded := make([][]byte, 0, len(encoded))
	for _, b64 := range encoded {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("decode base64: %w", err)
		}

		decoded = append(decoded, data)
	}

	return decoded, nil
}
