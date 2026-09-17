package crop

const DefaultSquarePadding = 32

// OptionsFor reports ok false when no crop was requested.
func OptionsFor(isCrop, isSquare bool, padding *int) (Options, bool) {
	if !isCrop && !isSquare {
		return Options{}, false
	}

	var value int

	switch {
	case padding != nil:
		value = *padding
	case isSquare:
		value = DefaultSquarePadding
	}

	return Options{IsSquare: isSquare, Padding: value}, true
}
