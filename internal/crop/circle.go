package crop

import "image"

const circleSamples = 4

// applyCircle makes every pixel outside the circle inscribed in dst transparent, the way a CSS 50% border radius
// would. Pixels the circle edge passes through keep a share of their alpha, so the outline stays smooth.
func applyCircle(dst *image.RGBA) {
	bounds := dst.Bounds()
	centerX := float64(bounds.Min.X) + float64(bounds.Dx())/2
	centerY := float64(bounds.Min.Y) + float64(bounds.Dy())/2
	radius := float64(min(bounds.Dx(), bounds.Dy())) / 2

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			applyCoverage(dst, x, y, circleCoverage(float64(x)-centerX, float64(y)-centerY, radius))
		}
	}
}

// circleCoverage reports the fraction of the pixel whose corner sits at (dx, dy) relative to the circle's centre
// that falls inside the circle, by supersampling.
func circleCoverage(dx, dy, radius float64) float64 {
	const (
		step   = 1.0 / circleSamples
		offset = step / 2
	)

	inside := 0

	for sampleY := range circleSamples {
		for sampleX := range circleSamples {
			x := dx + offset + float64(sampleX)*step
			y := dy + offset + float64(sampleY)*step

			if x*x+y*y <= radius*radius {
				inside++
			}
		}
	}

	return float64(inside) / (circleSamples * circleSamples)
}

// applyCoverage scales the premultiplied channels of a pixel by how much of it the circle covers.
func applyCoverage(dst *image.RGBA, x, y int, coverage float64) {
	if coverage >= 1 {
		return
	}

	c := dst.RGBAAt(x, y)
	c.R = scaleChannel(c.R, coverage)
	c.G = scaleChannel(c.G, coverage)
	c.B = scaleChannel(c.B, coverage)
	c.A = scaleChannel(c.A, coverage)

	dst.SetRGBA(x, y, c)
}

func scaleChannel(value uint8, factor float64) uint8 {
	return uint8(float64(value)*factor + 0.5)
}
