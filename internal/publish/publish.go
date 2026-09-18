package publish

import (
	"context"

	"github.com/wishmatic/neo-mcp/internal/s3upload"
	"github.com/wishmatic/neo-mcp/internal/shortener"
	"go.uber.org/zap"
)

type Publisher struct {
	uploader  *s3upload.Client
	shortener *shortener.Client
	log       *zap.Logger
}

func New(uploader *s3upload.Client, shortener *shortener.Client, log *zap.Logger) *Publisher {
	return &Publisher{uploader: uploader, shortener: shortener, log: log}
}

func (p *Publisher) Enabled() bool {
	return p.uploader != nil
}

// IsPublicURL reports whether rawURL already points at the world-readable namespace, so publishing it is a no-op.
func (p *Publisher) IsPublicURL(rawURL string) bool {
	if p.uploader == nil {
		return false
	}

	return p.uploader.IsPublicURL(rawURL)
}

func (p *Publisher) Images(ctx context.Context, label string, images [][]byte, contentType string, public, nsfw bool) ([]string, error) {
	urls := make([]string, 0, len(images))

	for _, data := range images {
		url, err := p.uploader.UploadFile(ctx, data, contentType, public, nsfw)
		if err != nil {
			p.log.Error(label+" upload to s3 failed", zap.Error(err))

			return nil, err
		}

		urls = append(urls, p.shorten(ctx, label, url))
	}

	return urls, nil
}

func (p *Publisher) File(ctx context.Context, label string, data []byte, contentType string, public bool) (string, error) {
	url, err := p.uploader.UploadFile(ctx, data, contentType, public, false)
	if err != nil {
		p.log.Error(label+" upload to s3 failed", zap.Error(err))

		return "", err
	}

	return p.shorten(ctx, label, url), nil
}

func (p *Publisher) shorten(ctx context.Context, label, url string) string {
	if p.shortener == nil {
		return url
	}

	short, err := p.shortener.Shorten(ctx, url)
	if err != nil {
		p.log.Warn(label+" url shortening failed; returning original url", zap.Error(err))

		return url
	}

	return short
}
