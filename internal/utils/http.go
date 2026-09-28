package utils

import (
	"fmt"
	"net/http"
	"net/url"
)

func IsHTTP(s string) bool {
	u, err := url.Parse(s)

	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func FmtHTTPErr(method, path string, resp *http.Response) error {
	body := ReadLimited(resp.Body)

	if body != "" {
		return fmt.Errorf("%s %s returned HTTP %d: %s", method, path, resp.StatusCode, body)
	}

	return fmt.Errorf("%s %s returned HTTP %d", method, path, resp.StatusCode)
}
