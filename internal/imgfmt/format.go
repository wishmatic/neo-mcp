package imgfmt

import (
	"fmt"
	"strings"
)

type Format string

const (
	PNG  Format = "png"
	JPEG Format = "jpeg"
	JXL  Format = "jxl"
	WebP Format = "webp"

	Default = WebP
)

func Parse(value string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(PNG):
		return PNG, nil
	case string(JPEG), "jpg":
		return JPEG, nil
	case string(JXL), "jpegxl", "jpeg-xl":
		return JXL, nil
	case string(WebP):
		return WebP, nil
	default:
		return "", fmt.Errorf("imgfmt: unknown format %q, want one of %s", value, strings.Join(Names(), ", "))
	}
}

func Names() []string {
	return []string{string(PNG), string(JPEG), string(JXL), string(WebP)}
}

func (f Format) String() string {
	return string(f)
}

func (f Format) MediaType() string {
	switch f {
	case PNG:
		return "image/png"
	case JPEG:
		return "image/jpeg"
	case JXL:
		return "image/jxl"
	case WebP:
		return "image/webp"
	default:
		return ""
	}
}

func (f Format) Extension() string {
	switch f {
	case PNG:
		return "png"
	case JPEG:
		return "jpg"
	case JXL:
		return "jxl"
	case WebP:
		return "webp"
	default:
		return ""
	}
}

func ExtensionForMediaType(mediaType string) string {
	for _, format := range []Format{PNG, JPEG, JXL, WebP} {
		if format.MediaType() == mediaType {
			return format.Extension()
		}
	}

	return "png"
}
