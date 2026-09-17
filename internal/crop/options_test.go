package crop

import "testing"

func TestOptionsFor(t *testing.T) {
	var (
		five = 5
		zero = 0
	)

	tests := []struct {
		name     string
		isCrop   bool
		isSquare bool
		padding  *int
		want     Options
		ok       bool
	}{
		{
			name: "crop disabled",
		},
		{
			name:   "crop only",
			isCrop: true,
			want:   Options{},
			ok:     true,
		},
		{
			name:     "square implies crop and defaults the padding",
			isSquare: true,
			want:     Options{IsSquare: true, Padding: DefaultSquarePadding},
			ok:       true,
		},
		{
			name:     "square with explicit padding",
			isCrop:   true,
			isSquare: true,
			padding:  &five,
			want:     Options{IsSquare: true, Padding: 5},
			ok:       true,
		},
		{
			name:    "crop with explicit padding",
			isCrop:  true,
			padding: &five,
			want:    Options{Padding: 5},
			ok:      true,
		},
		{
			name:     "square with zero padding",
			isCrop:   true,
			isSquare: true,
			padding:  &zero,
			want:     Options{IsSquare: true, Padding: 0},
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := OptionsFor(tt.isCrop, tt.isSquare, tt.padding)
			if ok != tt.ok {
				t.Fatalf("OptionsFor() ok = %v, want %v", ok, tt.ok)
			}

			if got != tt.want {
				t.Errorf("OptionsFor() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
