package mcp

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/bgkill"
	"github.com/wishmatic/neo-mcp/internal/imagegen"
	"github.com/wishmatic/neo-mcp/internal/novelai"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

func TestToolAnnotations(t *testing.T) {
	forge := sdwebui.New("http://example.com", false)

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: imagegen.New(forge, novelai.New("http://example.com", "sk", false)),
		Bgkill:    bgkill.New(forge),
		NovelAI:   novelai.New("http://example.com", "sk", false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	want := map[string]mcp.ToolAnnotations{
		"anlas":   {ReadOnlyHint: true, DestructiveHint: new(false), IdempotentHint: true, OpenWorldHint: new(true)},
		"txt2img": {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
		"img2img": {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
		"bgkill":  {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
	}

	for name, wantAnnotations := range want {
		t.Run(name, func(t *testing.T) {
			got := toolByName(t, srv, name).Annotations
			if got == nil {
				t.Fatal("annotations = nil, want all four hints set")
			}

			if got.ReadOnlyHint != wantAnnotations.ReadOnlyHint || got.IdempotentHint != wantAnnotations.IdempotentHint {
				t.Errorf("annotations = %+v, want readOnly=%t idempotent=%t",
					got, wantAnnotations.ReadOnlyHint, wantAnnotations.IdempotentHint)
			}

			assertBoolPtr(t, "destructiveHint", got.DestructiveHint, *wantAnnotations.DestructiveHint)
			assertBoolPtr(t, "openWorldHint", got.OpenWorldHint, *wantAnnotations.OpenWorldHint)

			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal annotations: %v", err)
			}

			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatalf("unmarshal annotations: %v", err)
			}

			for _, field := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
				if _, ok := fields[field].(bool); !ok {
					t.Errorf("annotations %s = %v, want an explicit boolean", field, fields[field])
				}
			}
		})
	}
}

func assertBoolPtr(t *testing.T, name string, got *bool, want bool) {
	t.Helper()

	if got == nil {
		t.Errorf("%s = nil, want %t", name, want)

		return
	}

	if *got != want {
		t.Errorf("%s = %t, want %t", name, *got, want)
	}
}
