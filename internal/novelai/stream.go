package novelai

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

const (
	finalEventType = "final"
	maxFrameBytes  = 64 << 20
)

var errNoFinalImage = errors.New("novelai stream ended without producing an image")

type streamEvent struct {
	EventType string `msgpack:"event_type"`
	Image     any    `msgpack:"image"`
}

func decodeFinalImage(r io.Reader) ([]byte, error) {
	var header [4]byte

	for {
		if _, err := io.ReadFull(r, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errNoFinalImage
			}

			return nil, fmt.Errorf("novelai: read frame header: %w", err)
		}

		size := int(binary.BigEndian.Uint32(header[:]))
		if size > maxFrameBytes {
			return nil, fmt.Errorf("novelai: frame length %d exceeds the %d byte cap", size, maxFrameBytes)
		}

		frame := make([]byte, size)
		if _, err := io.ReadFull(r, frame); err != nil {
			return nil, fmt.Errorf("novelai: read frame body: %w", err)
		}

		image, ok, err := decodeFrame(frame)
		if err != nil {
			return nil, err
		}

		if ok {
			return image, nil
		}
	}
}

func decodeFrame(frame []byte) ([]byte, bool, error) {
	var event streamEvent
	if err := msgpack.Unmarshal(frame, &event); err != nil {
		return nil, false, fmt.Errorf("novelai: decode frame: %w", err)
	}

	if event.EventType != finalEventType {
		return nil, false, nil
	}

	switch image := event.Image.(type) {
	case []byte:
		return image, true, nil
	case string:
		data, err := base64.StdEncoding.DecodeString(image)
		if err != nil {
			return nil, false, fmt.Errorf("novelai: decode base64 image: %w", err)
		}

		return data, true, nil
	default:
		return nil, false, nil
	}
}
