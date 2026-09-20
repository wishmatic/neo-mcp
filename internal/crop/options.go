package crop

const DefaultSquarePadding = 32

// Request is the crop as the caller asked for it, before defaults are applied.
type Request struct {
	IsCrop   bool
	IsSquare bool
	IsCircle bool
	Padding  *int
}

// OptionsFor reports ok false when no crop was requested. A circle implies a square canvas but, unlike a plain
// square, gets no padding by default: the circle is cut to the canvas edges.
func OptionsFor(req Request) (Options, bool) {
	isSquare := req.IsSquare || req.IsCircle
	if !req.IsCrop && !isSquare {
		return Options{}, false
	}

	opts := Options{IsSquare: isSquare, IsCircle: req.IsCircle}

	switch {
	case req.Padding != nil:
		opts.Padding = *req.Padding
	case isSquare && !req.IsCircle:
		opts.Padding = DefaultSquarePadding
	}

	return opts, true
}
