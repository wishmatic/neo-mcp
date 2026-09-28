package mcp

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/diffusion"
	"github.com/wishmatic/neo-mcp/internal/forge"
	"github.com/wishmatic/neo-mcp/internal/novelai"
)

func TestToolAnnotations(t *testing.T) {
	forgeClient := forge.New("http://example.com", zapNop())

	srv, err := New(Deps{
		Log:       zapNop(),
		Generator: diffusion.New(forgeClient, novelai.New("http://example.com", "sk")),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	want := map[string]mcp.ToolAnnotations{
		"txt2img": {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
		"img2img": {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
		"bgkill":  {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
		"edit":    {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
		"convert": {ReadOnlyHint: false, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)},
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
