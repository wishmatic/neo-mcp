package utils

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIsHTTP(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"//example.com", false},
		{"/relative", false},
		{"ftp://example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsHTTP(tt.in); got != tt.want {
			t.Errorf("IsHTTP(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "body included",
			status: http.StatusInternalServerError,
			body:   "boom",
			want:   "POST /sdapi/v1/txt2img returned HTTP 500: boom",
		},
		{name: "empty body", status: http.StatusNotFound, want: "POST /sdapi/v1/txt2img returned HTTP 404"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body))}

			if got := FmtHTTPErr(http.MethodPost, "/sdapi/v1/txt2img", resp).Error(); got != tt.want {
				t.Errorf("HTTPError() = %q, want %q", got, tt.want)
			}
		})
	}
}
