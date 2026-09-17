package novelai

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func TestDecodeFinalImageBin(t *testing.T) {
	stream := bytes.NewReader(encodeFrame(t, map[string]any{
		"event_type": "final",
		"image":      []byte("png"),
	}))

	image, err := decodeFinalImage(stream)
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "png" {
		t.Errorf("image = %q, want png", image)
	}
}

func TestDecodeFinalImageBase64(t *testing.T) {
	stream := bytes.NewReader(encodeFrame(t, map[string]any{
		"event_type": "final",
		"image":      base64.StdEncoding.EncodeToString([]byte("png")),
	}))

	image, err := decodeFinalImage(stream)
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "png" {
		t.Errorf("image = %q, want png", image)
	}
}

func TestDecodeFinalImageSkipsIntermediateFrames(t *testing.T) {
	stream := bytes.NewReader(bytes.Join([][]byte{
		encodeFrame(t, map[string]any{"event_type": "intermediate", "image": []byte("step")}),
		encodeFrame(t, map[string]any{"event_type": "final", "image": []byte("final")}),
	}, nil))

	image, err := decodeFinalImage(stream)
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "final" {
		t.Errorf("image = %q, want final", image)
	}
}

func TestDecodeFinalImageReturnsFirstFinalFrame(t *testing.T) {
	stream := bytes.NewReader(bytes.Join([][]byte{
		encodeFrame(t, map[string]any{"event_type": "final", "image": []byte("first")}),
		encodeFrame(t, map[string]any{"event_type": "final", "image": []byte("second")}),
	}, nil))

	image, err := decodeFinalImage(stream)
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "first" {
		t.Errorf("image = %q, want first", image)
	}
}

func TestDecodeFinalImageTruncatedFrame(t *testing.T) {
	frame := encodeFrame(t, map[string]any{"event_type": "final", "image": []byte("png")})

	if _, err := decodeFinalImage(bytes.NewReader(frame[:len(frame)-1])); err == nil {
		t.Fatal("decodeFinalImage() error = nil, want an error")
	}
}

func TestDecodeFinalImageEmptyStream(t *testing.T) {
	_, err := decodeFinalImage(bytes.NewReader(nil))

	if !errors.Is(err, errNoFinalImage) {
		t.Fatalf("error = %v, want errNoFinalImage", err)
	}

	if err.Error() != "novelai stream ended without producing an image" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestDecodeFinalImageWithoutFinalFrame(t *testing.T) {
	stream := bytes.NewReader(encodeFrame(t, map[string]any{
		"event_type": "intermediate",
		"image":      []byte("step"),
	}))

	if _, err := decodeFinalImage(stream); !errors.Is(err, errNoFinalImage) {
		t.Fatalf("error = %v, want errNoFinalImage", err)
	}
}

func TestDecodeFinalImageRejectsOversizedFrame(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, maxFrameBytes+1)

	_, err := decodeFinalImage(bytes.NewReader(header))
	if err == nil {
		t.Fatal("decodeFinalImage() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "cap") {
		t.Errorf("error = %q, want it to mention the cap", err.Error())
	}
}
