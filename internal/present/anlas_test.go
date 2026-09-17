package present

import "testing"

func TestAnlas(t *testing.T) {
	usage := 98

	tests := []struct {
		name         string
		total        int
		subscription int
		purchased    int
		usage        *int
		want         string
	}{
		{
			name:         "without usage",
			total:        10,
			subscription: 7,
			purchased:    3,
			want:         "Anlas: 10 (subscription 7, purchased 3)",
		},
		{
			name:         "with usage",
			total:        9993,
			subscription: 9961,
			purchased:    32,
			usage:        &usage,
			want:         "Anlas: 9993 (subscription 9961, purchased 32); V5 usage 98%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Anlas(tt.total, tt.subscription, tt.purchased, tt.usage); got != tt.want {
				t.Errorf("Anlas() = %q, want %q", got, tt.want)
			}
		})
	}
}
