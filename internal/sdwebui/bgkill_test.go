package sdwebui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type bgkillPayload struct {
	ModelName        string `json:"model_name"`
	Image            string `json:"image"`
	Resolution       string `json:"resolution"`
	ReturnForeground bool   `json:"return_foreground"`
	ReturnMask       bool   `json:"return_mask"`
	ReturnEdgeMask   bool   `json:"return_edge_mask"`
	SendOutput       bool   `json:"send_output"`
	UseFP16          bool   `json:"use_fp16"`
}

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

			c := New(server.URL, false)

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

			var payload bgkillPayload
			if err := json.Unmarshal(gotBody, &payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}

			if payload.ModelName != "General" {
				t.Errorf("model_name = %q, want General", payload.ModelName)
			}

			if !strings.HasPrefix(payload.Image, "data:image/png;base64,") {
				t.Errorf("image = %q, want a base64 data URI", payload.Image)
			}

			if payload.Resolution != "" {
				t.Errorf("resolution = %q, want empty for source size", payload.Resolution)
			}

			if !payload.ReturnForeground {
				t.Error("return_foreground = false, want true")
			}

			if payload.ReturnMask {
				t.Error("return_mask = true, want false")
			}

			if payload.ReturnEdgeMask {
				t.Error("return_edge_mask = true, want false")
			}

			if !payload.SendOutput {
				t.Error("send_output = false, want true")
			}

			if payload.UseFP16 != tt.wantUseFP16 {
				t.Errorf("use_fp16 = %v, want %v", payload.UseFP16, tt.wantUseFP16)
			}
		})
	}
}

func TestBgkillMissingForeground(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_image": ""}`))
	}))
	defer server.Close()

	c := New(server.URL, false)

	if _, err := c.Bgkill(context.Background(), BgkillRequest{ModelName: "General", ImageData: []byte("png")}); err == nil {
		t.Fatal("Bgkill() expected error, got nil")
	}
}
