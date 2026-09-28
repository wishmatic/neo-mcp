package utils

import "testing"

func TestBase64DataURI(t *testing.T) {
	if got := Encode([]byte("png")); got != "data:image/png;base64,cG5n" {
		t.Errorf("Base64DataURI() = %q, want %q", got, "data:image/png;base64,cG5n")
	}
}

func TestBase64DecodeAll(t *testing.T) {
	got, err := Decode([]string{"cG5n", "Z2lm"})
	if err != nil {
		t.Fatalf("Base64DecodeAll() error: %v", err)
	}

	want := []string{"png", "gif"}
	if len(got) != len(want) {
		t.Fatalf("Base64DecodeAll() returned %d images, want %d", len(got), len(want))
	}

	for i := range want {
		if string(got[i]) != want[i] {
			t.Errorf("Base64DecodeAll()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBase64DecodeAllEmpty(t *testing.T) {
	got, err := Decode(nil)
	if err != nil {
		t.Fatalf("Base64DecodeAll() error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("Base64DecodeAll(nil) = %v, want empty", got)
	}
}

func TestBase64DecodeAllInvalid(t *testing.T) {
	if _, err := Decode([]string{"cG5n", "not base64!"}); err == nil {
		t.Fatal("Base64DecodeAll() error = nil, want a decode error")
	}
}
