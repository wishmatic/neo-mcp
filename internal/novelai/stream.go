package novelai

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	finalEventType   = "final"
	errorEventType   = "error"
	maxFrameBytes    = 64 << 20
	streamMsgpack    = "msgpack"
	streamProbeBytes = 16
)

var errNoFinalImage = errors.New("novelai stream ended without producing an image")

type streamEvent struct {
	EventType string `msgpack:"event_type" json:"event_type"`
	Image     any    `msgpack:"image" json:"image"`
	Message   string `msgpack:"message" json:"message"`
}

// decodeFinalImage reads NovelAI's generation stream and returns the first final image. The stream is framed as
// length-prefixed msgpack unless the server picks Server-Sent Events instead, so the framing is detected up front.
func decodeFinalImage(r io.Reader) ([]byte, error) {
	buffered := bufio.NewReader(r)

	if isSSEStream(buffered) {
		return decodeSSEStream(buffered)
	}

	return decodeMsgpackStream(buffered)
}

func isSSEStream(r *bufio.Reader) bool {
	head, _ := r.Peek(streamProbeBytes)

	return bytes.Contains(head, []byte("data:")) || bytes.Contains(head, []byte("event:"))
}

func finalImage(event streamEvent) ([]byte, bool, error) {
	if event.EventType == errorEventType {
		return nil, false, fmt.Errorf("novelai: generation failed: %s", event.Message)
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
