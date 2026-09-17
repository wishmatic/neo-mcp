package resolve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

const (
	fetchTimeout = 60 * time.Second
	maxRedirects = 10
)

type ObjectStore interface {
	GetObject(ctx context.Context, key string) ([]byte, error)
}

type Resolver struct {
	store      ObjectStore
	publicBase *url.URL
	http       *http.Client
}

func New(store ObjectStore, publicBase string) (*Resolver, error) {
	r := &Resolver{
		store: store,
		http: &http.Client{
			Timeout: fetchTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	if publicBase != "" {
		base, err := url.Parse(publicBase)
		if err != nil || base.Scheme == "" || base.Host == "" {
			return nil, fmt.Errorf("resolve: invalid public base URL %q", publicBase)
		}

		r.publicBase = base
	}

	return r, nil
}

// Fetch returns the bytes for rawURL, following redirects.
//
// Garagefront URLs, including ones reached through a shortener redirect, are read straight from S3 by object key.
func (r *Resolver) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("resolve: parse URL %q: %w", rawURL, err)
	}

	for hop := 0; ; hop++ {
		if key, ok := r.objectKey(u); ok {
			if r.store == nil {
				return nil, fmt.Errorf("resolve: %s resolves to an S3 object but S3 is not configured", u)
			}

			return r.store.GetObject(ctx, key)
		}

		resp, err := r.get(ctx, u)
		if err != nil {
			return nil, err
		}

		if !isRedirect(resp) {
			return readResponse(u, resp)
		}

		location := resp.Header.Get("Location")
		resp.Body.Close()

		if location == "" {
			return nil, fmt.Errorf("resolve: %s returned HTTP %d without a Location header", u, resp.StatusCode)
		}

		if hop >= maxRedirects {
			return nil, fmt.Errorf("resolve: too many redirects fetching %q", rawURL)
		}

		next, err := u.Parse(location)
		if err != nil {
			return nil, fmt.Errorf("resolve: invalid redirect location %q: %w", location, err)
		}

		u = next
	}
}

func (r *Resolver) get(ctx context.Context, u *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("resolve: build request for %s: %w", u, err)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolve: fetch %s: %w", u, err)
	}

	return resp, nil
}

// objectKey mirrors Garagefront's path mapping: the URL path minus its leading slash is the object key, and only the
// "i"/"a" namespaces are servable.
func (r *Resolver) objectKey(u *url.URL) (string, bool) {
	if r.publicBase == nil || !strings.EqualFold(u.Host, r.publicBase.Host) {
		return "", false
	}

	segments := splitPath(u.Path)
	if len(segments) < 2 {
		return "", false
	}

	switch segments[0] {
	case "i", "a":
	default:
		return "", false
	}

	for _, segment := range segments {
		if segment == ".." {
			return "", false
		}
	}

	return strings.Join(segments, "/"), true
}

func splitPath(p string) []string {
	parts := strings.Split(p, "/")

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}

	return out
}

func isRedirect(resp *http.Response) bool {
	return resp.StatusCode >= 300 && resp.StatusCode < 400
}

func readResponse(u *url.URL, resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resolve: %s returned HTTP %d: %s", u, resp.StatusCode, utils.ReadLimited(resp.Body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("resolve: read %s: %w", u, err)
	}

	return data, nil
}
