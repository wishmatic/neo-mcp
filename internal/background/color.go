package background

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
)

type linear struct{ r, g, b float64 }

// ParseHex parses a hex RGB colour, with or without a leading '#', in six- or three-digit form, and returns it as an
// opaque colour.
func ParseHex(value string) (color.NRGBA, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(value), "#")

	if len(digits) == 3 {
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}

	if len(digits) != 6 {
		return color.NRGBA{}, fmt.Errorf("background: colour %q is not a hex RGB value", value)
	}

	raw, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("background: colour %q is not a hex RGB value: %w", value, err)
	}

	return color.NRGBA{R: uint8(raw >> 16), G: uint8(raw >> 8), B: uint8(raw), A: 0xff}, nil
}

func linearize(c color.NRGBA) linear {
	return linear{
		r: toLinear(float64(c.R) / 255),
		g: toLinear(float64(c.G) / 255),
		b: toLinear(float64(c.B) / 255),
	}
}

// mix moves each channel a fraction t of the way from the receiver toward other.
func (l linear) mix(other linear, t float64) linear {
	return linear{
		r: l.r + (other.r-l.r)*t,
		g: l.g + (other.g-l.g)*t,
		b: l.b + (other.b-l.b)*t,
	}
}

func (l linear) scale(factor float64) linear {
	return linear{r: l.r * factor, g: l.g * factor, b: l.b * factor}
}

func toLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}

	return math.Pow((v+0.055)/1.055, 2.4)
}

func toSRGB(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}

	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func clamp(value, low, high float64) float64 {
	return min(max(value, low), high)
}
