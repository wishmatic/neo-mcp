package imgfmt

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Format
	}{
		{in: "png", want: PNG},
		{in: "PNG", want: PNG},
		{in: "  png  ", want: PNG},
		{in: "jpeg", want: JPEG},
		{in: "jpg", want: JPEG},
		{in: "JPEG", want: JPEG},
		{in: "jxl", want: JXL},
		{in: "JPEGXL", want: JXL},
		{in: "jpeg-xl", want: JXL},
		{in: "webp", want: WebP},
		{in: "WebP", want: WebP},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}

			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseRejectsUnknown(t *testing.T) {
	for _, in := range []string{"", "  ", "gif", "image/png"} {
		t.Run(in, func(t *testing.T) {
			_, err := Parse(in)
			if err == nil {
				t.Fatalf("Parse(%q) error = nil, want an error", in)
			}

			for _, want := range Names() {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

func TestMediaTypeAndExtension(t *testing.T) {
	tests := map[Format]struct {
		mediaType string
		extension string
	}{
		PNG:  {mediaType: "image/png", extension: "png"},
		JPEG: {mediaType: "image/jpeg", extension: "jpg"},
		JXL:  {mediaType: "image/jxl", extension: "jxl"},
		WebP: {mediaType: "image/webp", extension: "webp"},
	}

	for format, want := range tests {
		if got := format.MediaType(); got != want.mediaType {
			t.Errorf("%s MediaType() = %q, want %q", format, got, want.mediaType)
		}

		if got := format.Extension(); got != want.extension {
			t.Errorf("%s Extension() = %q, want %q", format, got, want.extension)
		}
	}

	var unknown Format

	if got := unknown.MediaType(); got != "" {
		t.Errorf("zero MediaType() = %q, want empty", got)
	}

	if got := unknown.Extension(); got != "" {
		t.Errorf("zero Extension() = %q, want empty", got)
	}
}

func TestNames(t *testing.T) {
	want := []string{"png", "jpeg", "jxl", "webp"}
	got := Names()

	if len(got) != len(want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}

	for i, name := range want {
		if got[i] != name {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], name)
		}
	}

	if Default != WebP {
		t.Errorf("Default = %q, want %q", Default, WebP)
	}

	if Default.String() != "webp" {
		t.Errorf("Default.String() = %q, want webp", Default.String())
	}
}
