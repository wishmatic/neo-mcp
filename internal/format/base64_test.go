package format

import "testing"

func TestDecodeBase64(t *testing.T) {
	got, err := DecodeRawBase64("cG5n")
	if err != nil {
		t.Fatalf("DecodeBase64() error: %v", err)
	}

	if string(got) != "png" {
		t.Errorf("DecodeBase64() = %q, want %q", got, "png")
	}
}

func TestDecodeBase64Invalid(t *testing.T) {
	if _, err := DecodeRawBase64("not base64!"); err == nil {
		t.Fatal("DecodeBase64() error = nil, want a decode error")
	}
}
