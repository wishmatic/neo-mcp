package forge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestBgkillPayload(t *testing.T) {
	tests := []struct {
		name        string
		isFullMode  bool
		wantUseFP16 bool
	}{
		{name: "full mode off", isFullMode: false, wantUseFP16: true},
		{name: "full mode on", isFullMode: true, wantUseFP16: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotPath string
				gotBody []byte
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"output_image": "` + base64.StdEncoding.EncodeToString([]byte("foreground")) + `"}`))
			}))
			defer server.Close()

			c := New(server.URL, zap.NewNop())

			out, err := c.Bgkill(context.Background(), BgkillRequest{
				ModelName:  "General",
				ImageData:  []byte("png"),
				IsFullMode: tt.isFullMode,
			})
			if err != nil {
				t.Fatalf("Bgkill() error: %v", err)
			}

			if string(out) != "foreground" {
				t.Errorf("output = %q, want foreground", out)
			}

			if gotPath != "/birefnet/single" {
				t.Errorf("path = %q, want /birefnet/single", gotPath)
			}

			want := map[string]any{
				"model_name":        "General",
				"image":             base64.StdEncoding.EncodeToString([]byte("png")),
				"resolution":        "",
				"return_foreground": true,
				"return_mask":       false,
				"return_edge_mask":  false,
				"send_output":       true,
				"use_fp16":          tt.wantUseFP16,
			}

			var got map[string]any
			if err := json.Unmarshal(gotBody, &got); err != nil {
				t.Fatalf("decode payload: %v", err)
			}

			if !maps.Equal(got, want) {
				t.Errorf("payload = %v, want %v", got, want)
			}
		})
	}
}

func TestBgkillMissingForeground(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, reader *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"output_image": ""}`))
		}),
	)
	defer server.Close()

	client := New(server.URL, zap.NewNop())

	if _, err := client.Bgkill(
		context.Background(), BgkillRequest{ModelName: "General", ImageData: []byte("png")},
	); err == nil {
		t.Fatal("Bgkill() expected error, got nil")
	}
}
