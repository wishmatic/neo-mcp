package filestore

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

var mediaTypes = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"webp": "image/webp",
	"jxl":  "image/jxl",
}

func (c *Client) Register(r chi.Router) {
	r.Get("/i/*", c.serve)
	r.Head("/i/*", c.serve)
}

func (c *Client) serve(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")

	mediaType, ok := mediaTypeFor(key)
	if !ok {
		http.NotFound(w, r)

		return
	}

	path, err := c.safePath(key)
	if err != nil {
		http.NotFound(w, r)

		return
	}

	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)

		return
	}

	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func mediaTypeFor(key string) (string, bool) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(key), "."))

	mediaType, ok := mediaTypes[ext]

	return mediaType, ok
}
