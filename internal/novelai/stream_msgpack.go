package novelai

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

func decodeMsgpackStream(r io.Reader) ([]byte, error) {
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

		var event streamEvent
		if err := msgpack.Unmarshal(frame, &event); err != nil {
			return nil, fmt.Errorf("novelai: decode frame: %w", err)
		}

		image, ok, err := finalImage(event)
		if err != nil {
			return nil, err
		}

		if ok {
			return image, nil
		}
	}
}
