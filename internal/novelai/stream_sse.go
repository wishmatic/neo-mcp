package novelai

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const sseDataField = "data:"

func decodeSSEStream(r *bufio.Reader) ([]byte, error) {
	var data []string

	for {
		line, readErr := r.ReadString('\n')
		ended := errors.Is(readErr, io.EOF)
		text := strings.TrimRight(line, "\r\n")

		if strings.HasPrefix(text, sseDataField) {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(text, sseDataField), " "))
		}

		if text == "" || ended {
			image, ok, err := decodeSSEEvent(data)
			if err != nil {
				return nil, err
			}

			if ok {
				return image, nil
			}

			data = data[:0]
		}

		if ended {
			return nil, errNoFinalImage
		}

		if readErr != nil {
			return nil, fmt.Errorf("novelai: read sse stream: %w", readErr)
		}
	}
}

func decodeSSEEvent(data []string) ([]byte, bool, error) {
	if len(data) == 0 {
		return nil, false, nil
	}

	var event streamEvent
	if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &event); err != nil {
		return nil, false, fmt.Errorf("novelai: decode sse event: %w", err)
	}

	return finalImage(event)
}
