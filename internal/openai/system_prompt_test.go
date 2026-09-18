package openai

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestDefaultSystemPromptMatchesFile(t *testing.T) {
	want, err := os.ReadFile("system_prompt.txt")
	if err != nil {
		t.Fatalf("read system_prompt.txt: %v", err)
	}

	if DefaultSystemPrompt != string(want) {
		t.Errorf("DefaultSystemPrompt = %q, want the contents of system_prompt.txt", DefaultSystemPrompt)
	}
}

func TestDescribeUsesBuiltInSystemPromptWhenUnset(t *testing.T) {
	server, captured := newCaptureServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	c := newClient(t, Config{BaseURL: server.URL, Model: "m"})

	if _, err := c.Describe(context.Background(), DescribeRequest{ImageData: []byte("x"), MediaType: "image/png"}); err != nil {
		t.Fatalf("Describe() error: %v", err)
	}

	if want := strings.TrimSpace(DefaultSystemPrompt); want != "" {
		if got := systemMessage(t, captured.body); got != want {
			t.Errorf("system content = %q, want the built-in default", got)
		}

		return
	}

	if payload := decodePayload(t, captured.body); len(payload.Messages) != 1 || payload.Messages[0].Role != "user" {
		t.Errorf("messages = %+v, want only the user message while the built-in default is empty", payload.Messages)
	}
}
