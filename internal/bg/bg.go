package bg

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand/v2"
)

// Ramp is how far each end of the gradient moves from the base colour, as a fraction of its linear-light value. The
// ends scale by 1-Ramp and 1+Ramp, so the middle of the ramp stays the base colour. It is deliberately small: the
// backdrop is meant to be subtle.
const Ramp = 0.15

// RandomAngle returns a ramp direction in radians.
func RandomAngle(rng *rand.Rand) float64 {
	return rng.Float64() * 2 * math.Pi
}

// Compose returns an opaque gradient the size of src, ramping from a lighter to a darker variant of base along angle,
// with src drawn over it. dither breaks up the banding a subtle ramp would otherwise show once quantised.
func Compose(src image.Image, base color.NRGBA, angle float64, dither *rand.Rand) *image.RGBA {
	bounds := src.Bounds()

	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	fillRamp(dst, base, angle, dither)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)

	return dst
}

func fillRamp(dst *image.RGBA, base color.NRGBA, angle float64, dither *rand.Rand) {
	baseLinear := linearize(base)
	light := baseLinear.scale(1 + Ramp)
	dark := baseLinear.scale(1 - Ramp)

	dx, dy := math.Cos(angle), math.Sin(angle)
	start, end := rampExtent(float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy()), dx, dy)
	span := end - start

	for y := range dst.Bounds().Dy() {
		for x := range dst.Bounds().Dx() {
			u := 0.0
			if span != 0 {
				u = ((float64(x)+0.5)*dx + (float64(y)+0.5)*dy - start) / span
			}

			dst.SetRGBA(x, y, encode(light.mix(dark, u), dither))
		}
	}
}

// rampExtent is the range the pixel-centre projection spans, taken over the canvas corners.
func rampExtent(width, height, dx, dy float64) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)

	for _, corner := range [4][2]float64{
		{0.5, 0.5},
		{width - 0.5, 0.5},
		{0.5, height - 0.5},
		{width - 0.5, height - 0.5},
	} {
		projection := corner[0]*dx + corner[1]*dy
		lo = min(lo, projection)
		hi = max(hi, projection)
	}

	return lo, hi
}

func encode(c linear, dither *rand.Rand) color.RGBA {
	return color.RGBA{
		R: quantize(c.r, dither),
		G: quantize(c.g, dither),
		B: quantize(c.b, dither),
		A: 0xff,
	}
}

// quantize converts a linear channel to 8-bit sRGB, adding triangular noise of about one step so the ramp comes out
// dithered rather than banded.
func quantize(channel float64, dither *rand.Rand) uint8 {
	value := toSRGB(clamp(channel, 0, 1))*255 + dither.Float64() - dither.Float64()

	return uint8(math.Round(clamp(value, 0, 255)))
}
