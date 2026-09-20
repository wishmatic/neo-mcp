package crop

import "testing"

func TestOptionsFor(t *testing.T) {
	var (
		five = 5
		zero = 0
	)

	tests := []struct {
		name string
		req  Request
		want Options
		ok   bool
	}{
		{
			name: "crop disabled",
		},
		{
			name: "crop only",
			req:  Request{IsCrop: true},
			want: Options{},
			ok:   true,
		},
		{
			name: "square implies crop and defaults the padding",
			req:  Request{IsSquare: true},
			want: Options{IsSquare: true, Padding: DefaultSquarePadding},
			ok:   true,
		},
		{
			name: "square with explicit padding",
			req:  Request{IsCrop: true, IsSquare: true, Padding: &five},
			want: Options{IsSquare: true, Padding: 5},
			ok:   true,
		},
		{
			name: "crop with explicit padding",
			req:  Request{IsCrop: true, Padding: &five},
			want: Options{Padding: 5},
			ok:   true,
		},
		{
			name: "square with zero padding",
			req:  Request{IsCrop: true, IsSquare: true, Padding: &zero},
			want: Options{IsSquare: true, Padding: 0},
			ok:   true,
		},
		{
			name: "circle implies square and is cut to the edges",
			req:  Request{IsCircle: true},
			want: Options{IsSquare: true, IsCircle: true},
			ok:   true,
		},
		{
			name: "circle with square keeps no padding",
			req:  Request{IsSquare: true, IsCircle: true},
			want: Options{IsSquare: true, IsCircle: true},
			ok:   true,
		},
		{
			name: "circle with explicit padding",
			req:  Request{IsCrop: true, IsCircle: true, Padding: &five},
			want: Options{IsSquare: true, IsCircle: true, Padding: 5},
			ok:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := OptionsFor(tt.req)
			if ok != tt.ok {
				t.Fatalf("OptionsFor() ok = %v, want %v", ok, tt.ok)
			}

			if got != tt.want {
				t.Errorf("OptionsFor() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
