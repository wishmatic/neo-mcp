package novelai

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func sseFinalEvent(t *testing.T, image []byte) string {
	t.Helper()

	return `event: final
data: {"event_type":"final","image":"` + base64.StdEncoding.EncodeToString(image) + `"}

`
}

func TestDecodeFinalImageSSEFinalEvent(t *testing.T) {
	image, err := decodeFinalImage(strings.NewReader(sseFinalEvent(t, []byte("png"))))
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "png" {
		t.Errorf("image = %q, want png", image)
	}
}

func TestDecodeFinalImageSSESkipsIntermediateEvents(t *testing.T) {
	stream := `event: intermediate
data: {"event_type":"intermediate","image":"c3RlcA=="}

` + sseFinalEvent(t, []byte("final"))

	image, err := decodeFinalImage(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "final" {
		t.Errorf("image = %q, want final", image)
	}
}

func TestDecodeFinalImageSSEMultilineData(t *testing.T) {
	stream := `data: {"event_type":"final",
data: "image":"` + base64.StdEncoding.EncodeToString([]byte("png")) + `"}

`

	image, err := decodeFinalImage(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "png" {
		t.Errorf("image = %q, want png", image)
	}
}

func TestDecodeFinalImageSSEWithoutTrailingBlankLine(t *testing.T) {
	stream := strings.TrimRight(sseFinalEvent(t, []byte("png")), "\n")

	image, err := decodeFinalImage(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("decodeFinalImage() error: %v", err)
	}

	if string(image) != "png" {
		t.Errorf("image = %q, want png", image)
	}
}

func TestDecodeFinalImageSSEWithoutFinalEvent(t *testing.T) {
	stream := `event: intermediate
data: {"event_type":"intermediate","image":"c3RlcA=="}

`

	if _, err := decodeFinalImage(strings.NewReader(stream)); !errors.Is(err, errNoFinalImage) {
		t.Fatalf("error = %v, want errNoFinalImage", err)
	}
}

func TestDecodeFinalImageSSEErrorEvent(t *testing.T) {
	stream := `event: error
data: {"event_type":"error","message":"bad init image"}

`

	_, err := decodeFinalImage(strings.NewReader(stream))
	if err == nil {
		t.Fatal("decodeFinalImage() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "bad init image") {
		t.Errorf("error = %q, want the server message", err.Error())
	}
}
