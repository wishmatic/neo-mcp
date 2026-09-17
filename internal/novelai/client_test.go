package novelai

import "testing"

func TestNewTrimsBaseURL(t *testing.T) {
	client := New("https://image.novelai.net/", "sk-x", false)

	if client.baseURL != "https://image.novelai.net" {
		t.Errorf("baseURL = %q, want https://image.novelai.net", client.baseURL)
	}

	if client.apiKey != "sk-x" {
		t.Errorf("apiKey = %q, want sk-x", client.apiKey)
	}
}

func TestNewAcceptsEmptyKey(t *testing.T) {
	if client := New("http://example.com", "", false); client.apiKey != "" {
		t.Errorf("apiKey = %q, want empty", client.apiKey)
	}
}

func TestResolveSeed(t *testing.T) {
	client := New("http://example.com", "k", false)
	client.randomSeed = func() uint32 { return 7 }

	if got := client.resolveSeed(-1); got != 7 {
		t.Errorf("resolveSeed(-1) = %d, want the stubbed 7", got)
	}

	if got := client.resolveSeed(42); got != 42 {
		t.Errorf("resolveSeed(42) = %d, want 42", got)
	}
}
