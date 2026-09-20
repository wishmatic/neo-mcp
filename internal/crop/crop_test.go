package crop

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func newImage(width, height int) *image.NRGBA {
	return image.NewNRGBA(image.Rect(0, 0, width, height))
}

func fill(img *image.NRGBA, r image.Rectangle, alpha uint8) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: alpha})
		}
	}
}

func encode(t *testing.T, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func mustCrop(t *testing.T, img image.Image, opts Options) image.Image {
	t.Helper()

	out, err := ToContent(encode(t, img), opts)
	if err != nil {
		t.Fatalf("ToContent() error: %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	return decoded
}

func alphaAt(img image.Image, x, y int) uint8 {
	_, _, _, alpha := img.At(x, y).RGBA()

	return uint8(alpha >> 8)
}

func opaqueAt(img image.Image, x, y int) bool {
	_, _, _, alpha := img.At(x, y).RGBA()

	return alpha == 0xffff
}

func transparentAt(img image.Image, x, y int) bool {
	_, _, _, alpha := img.At(x, y).RGBA()

	return alpha == 0
}

func TestToContentCropsToForeground(t *testing.T) {
	src := newImage(10, 10)
	fill(src, image.Rect(3, 4, 6, 8), 255)

	out := mustCrop(t, src, Options{})

	if got, want := out.Bounds(), image.Rect(0, 0, 3, 4); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	for _, p := range []image.Point{{0, 0}, {2, 0}, {0, 3}, {2, 3}} {
		if !opaqueAt(out, p.X, p.Y) {
			t.Errorf("pixel %v is transparent, want opaque", p)
		}
	}

	want := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	if got := color.NRGBAModel.Convert(out.At(0, 0)).(color.NRGBA); got != want {
		t.Errorf("pixel = %v, want %v", got, want)
	}
}

func TestToContentThreshold(t *testing.T) {
	src := newImage(6, 6)
	fill(src, image.Rect(1, 1, 3, 3), 255)
	fill(src, image.Rect(5, 5, 6, 6), 4)

	tests := []struct {
		name      string
		threshold uint8
		want      image.Rectangle
	}{
		{name: "any visible pixel", threshold: 0, want: image.Rect(0, 0, 5, 5)},
		{name: "noise below threshold", threshold: 8, want: image.Rect(0, 0, 2, 2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := mustCrop(t, src, Options{Threshold: tt.threshold})

			if got := out.Bounds(); got != tt.want {
				t.Errorf("bounds = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToContentPadding(t *testing.T) {
	src := newImage(5, 5)
	fill(src, image.Rect(1, 1, 3, 3), 255)

	out := mustCrop(t, src, Options{Padding: 2})

	if got, want := out.Bounds(), image.Rect(0, 0, 6, 6); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	if !transparentAt(out, 0, 0) || !transparentAt(out, 1, 1) {
		t.Error("expected transparent padding at (0,0) and (1,1)")
	}

	if !opaqueAt(out, 2, 2) || !opaqueAt(out, 3, 3) {
		t.Error("expected the content to start at (2,2)")
	}
}

func TestToContentSquare(t *testing.T) {
	src := newImage(6, 6)
	fill(src, image.Rect(1, 1, 3, 5), 255)

	out := mustCrop(t, src, Options{IsSquare: true})

	if got, want := out.Bounds(), image.Rect(0, 0, 4, 4); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	if !opaqueAt(out, 1, 0) || !opaqueAt(out, 2, 3) {
		t.Error("content is not centered in the square")
	}

	if !transparentAt(out, 0, 0) || !transparentAt(out, 3, 0) {
		t.Error("expected transparent margins on the square's sides")
	}
}

func TestToContentSquareWithPadding(t *testing.T) {
	src := newImage(6, 6)
	fill(src, image.Rect(1, 1, 3, 5), 255)

	out := mustCrop(t, src, Options{IsSquare: true, Padding: 1})

	if got, want := out.Bounds(), image.Rect(0, 0, 6, 6); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	if !opaqueAt(out, 2, 1) || !opaqueAt(out, 3, 4) {
		t.Error("content is not centered in the padded square")
	}

	if !transparentAt(out, 1, 1) || !transparentAt(out, 4, 1) || !transparentAt(out, 2, 0) {
		t.Error("expected transparent padding around the content")
	}
}

func TestToContentCircle(t *testing.T) {
	src := newImage(20, 20)
	fill(src, image.Rect(2, 2, 18, 18), 255)

	out := mustCrop(t, src, Options{IsSquare: true, IsCircle: true})

	if got, want := out.Bounds(), image.Rect(0, 0, 16, 16); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	for _, p := range []image.Point{{0, 0}, {15, 0}, {0, 15}, {15, 15}} {
		if !transparentAt(out, p.X, p.Y) {
			t.Errorf("corner %v is visible, want it cut off by the circle", p)
		}
	}

	for _, p := range []image.Point{{8, 8}, {0, 7}, {0, 8}, {7, 0}} {
		if !opaqueAt(out, p.X, p.Y) {
			t.Errorf("pixel %v is not opaque, want it inside the circle", p)
		}
	}

	want := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	if got := color.NRGBAModel.Convert(out.At(8, 8)).(color.NRGBA); got != want {
		t.Errorf("pixel = %v, want %v", got, want)
	}

	if alpha := alphaAt(out, 2, 2); alpha == 0 || alpha == 255 {
		t.Errorf("edge pixel alpha = %d, want a partial alpha for an antialiased outline", alpha)
	}

	if got, want := alphaAt(out, 2, 2), alphaAt(out, 13, 13); got != want {
		t.Errorf("alpha = %d and %d, want the circle to be symmetric", got, want)
	}
}

func TestToContentCircleWithPadding(t *testing.T) {
	src := newImage(6, 6)
	fill(src, image.Rect(1, 1, 5, 5), 255)

	out := mustCrop(t, src, Options{IsSquare: true, IsCircle: true, Padding: 2})

	if got, want := out.Bounds(), image.Rect(0, 0, 8, 8); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	if !transparentAt(out, 0, 0) || !transparentAt(out, 7, 0) {
		t.Error("expected the circle to leave the corners transparent")
	}

	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			if !opaqueAt(out, x, y) {
				t.Errorf("pixel (%d, %d) was cut off, want the padding to clear the circle", x, y)
			}
		}
	}
}

func TestToContentErrors(t *testing.T) {
	src := newImage(4, 4)

	tests := []struct {
		name string
		data []byte
		opts Options
	}{
		{name: "no visible content", data: encode(t, src)},
		{name: "negative padding", data: encode(t, src), opts: Options{Padding: -1}},
		{name: "not a png", data: []byte("not a png")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ToContent(tt.data, tt.opts); err == nil {
				t.Fatal("ToContent() expected error, got nil")
			}
		})
	}
}
