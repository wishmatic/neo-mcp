package filestore

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"go.uber.org/zap"
)

const (
	namespace   = "i"
	nsfwSegment = "nsfw"
	dirMode     = 0o750
	fileMode    = 0o640
)

type Config struct {
	Dir           string
	PublicBase    *url.URL
	RetentionDays int
}

type Client struct {
	cfg Config
	log *zap.Logger
}

func New(cfg Config, log *zap.Logger) (*Client, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("filestore: a storage directory is required")
	}

	if cfg.PublicBase == nil || cfg.PublicBase.Host == "" {
		return nil, fmt.Errorf("filestore: a public base URL is required")
	}

	if err := os.MkdirAll(cfg.Dir, dirMode); err != nil {
		return nil, fmt.Errorf("filestore: create %s: %w", cfg.Dir, err)
	}

	return &Client{cfg: cfg, log: log}, nil
}

func (c *Client) UploadFile(ctx context.Context, data []byte, contentType string, nsfw bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	key := objectKey(nsfw, imgfmt.ExtensionForMediaType(contentType))

	path, err := c.safePath(key)
	if err != nil {
		return "", err
	}

	if err := writeFileAtomic(path, data); err != nil {
		return "", fmt.Errorf("filestore: store %s: %w", key, err)
	}

	c.log.Info("stored image",
		zap.String("key", key),
		zap.String("content_type", contentType),
		zap.Int("bytes", len(data)),
	)

	return c.url(key), nil
}

func (c *Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path, err := c.safePath(key)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("filestore: read %s: %w", key, err)
	}

	return data, nil
}

func (c *Client) url(key string) string {
	base := *c.cfg.PublicBase
	base.Path = strings.TrimSuffix(base.Path, "/") + "/" + key

	return base.String()
}

func objectKey(nsfw bool, ext string) string {
	parts := []string{namespace}
	if nsfw {
		parts = append(parts, nsfwSegment)
	}

	parts = append(parts, time.Now().UTC().Format("2006-01"), uuid.NewString()+"."+ext)

	return strings.Join(parts, "/")
}

func (c *Client) safePath(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("filestore: empty key")
	}

	if strings.HasPrefix(key, "/") || filepath.IsAbs(key) {
		return "", fmt.Errorf("filestore: key %q is absolute", key)
	}

	segments := strings.Split(key, "/")
	if segments[0] != namespace {
		return "", fmt.Errorf("filestore: key %q is outside the %s namespace", key, namespace)
	}

	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("filestore: key %q has an invalid segment", key)
		}
	}

	full := filepath.Join(c.cfg.Dir, filepath.FromSlash(key))

	rel, err := filepath.Rel(c.cfg.Dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("filestore: key %q escapes the storage directory", key)
	}

	return full, nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	tmpName = ""

	return nil
}
