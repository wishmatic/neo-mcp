package bgkill

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/crop"
	"github.com/wishmatic/neo-mcp/internal/sdwebui"
)

func newForge(t *testing.T, foreground []byte) (*sdwebui.Client, *map[string]any) {
	t.Helper()

	captured := &map[string]any{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		*captured = body

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"output_image": base64.StdEncoding.EncodeToString(foreground),
		})
	}))

	t.Cleanup(server.Close)

	return sdwebui.New(server.URL, false), captured
}

func pngWithContent(t *testing.T, size, from, to int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, size, size))

	for y := from; y < to; y++ {
		for x := from; x < to; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func decodeSize(t *testing.T, data []byte) image.Rectangle {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	return img.Bounds()
}

func TestEnabled(t *testing.T) {
	if New(nil).Enabled() {
		t.Error("Enabled() = true without a Forge client, want false")
	}

	forge, _ := newForge(t, pngWithContent(t, 4, 1, 2))

	if !New(forge).Enabled() {
		t.Error("Enabled() = false with a Forge client, want true")
	}
}

func TestRemoveReturnsForeground(t *testing.T) {
	foreground := pngWithContent(t, 10, 2, 4)
	forge, captured := newForge(t, foreground)

	got, err := New(forge).Remove(context.Background(), Request{
		ModelName:  "Portrait",
		ImageData:  []byte("input"),
		IsFullMode: true,
	})
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if !bytes.Equal(got, foreground) {
		t.Error("Remove() did not return the foreground unchanged without a crop")
	}

	if (*captured)["model_name"] != "Portrait" {
		t.Errorf("model_name = %v, want Portrait", (*captured)["model_name"])
	}

	if (*captured)["use_fp16"] != false {
		t.Errorf("use_fp16 = %v, want false in full mode", (*captured)["use_fp16"])
	}
}

func TestRemoveCropsToContent(t *testing.T) {
	forge, _ := newForge(t, pngWithContent(t, 10, 2, 4))

	got, err := New(forge).Remove(context.Background(), Request{ModelName: "General", IsCrop: true})
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if bounds := decodeSize(t, got); bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Errorf("bounds = %v, want a 2x2 crop", bounds)
	}
}

func TestRemoveSquaresWithDefaultPadding(t *testing.T) {
	forge, _ := newForge(t, pngWithContent(t, 10, 2, 4))

	got, err := New(forge).Remove(context.Background(), Request{ModelName: "General", IsSquare: true})
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	want := 2 + 2*crop.DefaultSquarePadding

	if bounds := decodeSize(t, got); bounds.Dx() != want || bounds.Dy() != want {
		t.Errorf("bounds = %v, want %dx%d", bounds, want, want)
	}
}
