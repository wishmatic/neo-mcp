package publish

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

type Store interface {
	UploadFile(ctx context.Context, data []byte, contentType string, nsfw bool) (string, error)
}

type Publisher struct {
	store Store
	log   *zap.Logger
}

func New(store Store, log *zap.Logger) *Publisher {
	return &Publisher{store: store, log: log}
}

func (p *Publisher) Enabled() bool {
	return p.store != nil
}

func (p *Publisher) Images(ctx context.Context, label string, images [][]byte, contentType string, nsfw bool) ([]string, error) {
	if p.store == nil {
		return nil, errors.New("publish: image storage is not configured")
	}

	urls := make([]string, 0, len(images))

	for _, data := range images {
		url, err := p.store.UploadFile(ctx, data, contentType, nsfw)
		if err != nil {
			p.log.Error(label+" upload failed", zap.Error(err))

			return nil, err
		}

		urls = append(urls, url)
	}

	return urls, nil
}
