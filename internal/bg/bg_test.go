package bg

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"testing"
)

func newRNG() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func transparentSource(size int) *image.NRGBA {
	return image.NewNRGBA(image.Rect(0, 0, size, size))
}

func opaqueSource(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := range size {
		for x := range size {
			img.SetNRGBA(x, y, color.NRGBA{R: 10, G: 200, B: 30, A: 0xff})
		}
	}

	return img
}

func luminance(img image.Image, x, y int) uint32 {
	r, g, b, _ := img.At(x, y).RGBA()

	return (r*299 + g*587 + b*114) / 1000
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}

	return int(b - a)
}

func TestParseHex(t *testing.T) {
	blue := color.NRGBA{R: 0x4a, G: 0x6f, B: 0xa5, A: 0xff}

	tests := []struct {
		name  string
		value string
		want  color.NRGBA
		ok    bool
	}{
		{name: "six digits with hash", value: "#4a6fa5", want: blue, ok: true},
		{name: "six digits without hash", value: "4a6fa5", want: blue, ok: true},
		{name: "uppercase", value: "#4A6FA5", want: blue, ok: true},
		{name: "surrounding space", value: "  #4a6fa5 ", want: blue, ok: true},
		{name: "shorthand with hash", value: "#39c", want: color.NRGBA{R: 0x33, G: 0x99, B: 0xcc, A: 0xff}, ok: true},
		{name: "shorthand without hash", value: "39c", want: color.NRGBA{R: 0x33, G: 0x99, B: 0xcc, A: 0xff}, ok: true},
		{name: "empty", value: ""},
		{name: "hash only", value: "#"},
		{name: "too short", value: "#12345"},
		{name: "too long", value: "#1234567"},
		{name: "not hex", value: "#12345g"},
		{name: "four digits", value: "1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHex(tt.value)

			if !tt.ok {
				if err == nil {
					t.Fatalf("ParseHex(%q) error = nil, want an error", tt.value)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseHex(%q) error: %v", tt.value, err)
			}

			if got != tt.want {
				t.Errorf("ParseHex(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestRandomAngleInRangeAndVaries(t *testing.T) {
	rng := newRNG()
	seen := map[float64]bool{}

	for range 1000 {
		angle := RandomAngle(rng)
		if angle < 0 || angle >= 2*math.Pi {
			t.Fatalf("angle = %v, want it in [0, 2pi)", angle)
		}

		seen[angle] = true
	}

	if len(seen) < 2 {
		t.Error("RandomAngle returned the same angle every time")
	}
}

func TestComposeSizeAndOpacity(t *testing.T) {
	base, err := ParseHex("#4a6fa5")
	if err != nil {
		t.Fatalf("ParseHex() error: %v", err)
	}

	out := Compose(transparentSource(16), base, 0, newRNG())

	if got, want := out.Bounds(), image.Rect(0, 0, 16, 16); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	for y := range 16 {
		for x := range 16 {
			if _, _, _, alpha := out.At(x, y).RGBA(); alpha != 0xffff {
				t.Fatalf("pixel (%d, %d) alpha = %d, want opaque", x, y, alpha>>8)
			}
		}
	}
}

func TestComposeIsDeterministic(t *testing.T) {
	base, _ := ParseHex("#4a6fa5")

	first := Compose(transparentSource(24), base, 1.234, newRNG())
	second := Compose(transparentSource(24), base, 1.234, newRNG())

	if !bytes.Equal(first.Pix, second.Pix) {
		t.Error("Compose with the same angle and seed produced different pixels")
	}
}

func TestComposeRampsAlongAngle(t *testing.T) {
	base, _ := ParseHex("#4a6fa5")

	out := Compose(transparentSource(32), base, 0, newRNG())

	if left, right := luminance(out, 0, 16), luminance(out, 31, 16); left <= right {
		t.Errorf("luminance left = %d, right = %d, want the ramp lighter where the angle starts", left, right)
	}
}

func TestComposeKeepsBaseColourAtTheMiddle(t *testing.T) {
	base, _ := ParseHex("#4a6fa5")

	out := Compose(transparentSource(5), base, 0, newRNG())
	got := color.NRGBAModel.Convert(out.At(2, 2)).(color.NRGBA)

	for _, channel := range []struct {
		name string
		got  uint8
		want uint8
	}{
		{name: "red", got: got.R, want: base.R},
		{name: "green", got: got.G, want: base.G},
		{name: "blue", got: got.B, want: base.B},
	} {
		if diff := absDiff(channel.got, channel.want); diff > 3 {
			t.Errorf("%s = %d, want within 3 of %d", channel.name, channel.got, channel.want)
		}
	}
}

func TestComposeDrawsOpaqueSourceOverTheGradient(t *testing.T) {
	base, _ := ParseHex("#4a6fa5")
	src := opaqueSource(8)

	out := Compose(src, base, 0.7, newRNG())

	for y := range 8 {
		for x := range 8 {
			if got := color.NRGBAModel.Convert(out.At(x, y)).(color.NRGBA); got != (color.NRGBA{R: 10, G: 200, B: 30, A: 0xff}) {
				t.Fatalf("pixel (%d, %d) = %v, want the opaque source", x, y, got)
			}
		}
	}
}
