package s3upload

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (u *Client) UploadImage(ctx context.Context, data []byte) (string, error) {
	key := fmt.Sprintf("%s/%s.png", time.Now().UTC().Format("2006-01"), uuid.NewString())

	u.log.Info("uploading image", zap.String("key", key), zap.Int("bytes", len(data)))

	_, err := u.writer.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		u.log.Error("s3 put object failed",
			zap.String("bucket", u.cfg.Bucket),
			zap.String("key", key),
			zap.Error(err),
		)

		return "", fmt.Errorf("uploading image to s3://%s/%s: %w", u.cfg.Bucket, key, err)
	}

	presigner := s3.NewPresignClient(u.reader)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(u.cfg.PresignExpiry))
	if err != nil {
		u.log.Error("s3 presign failed",
			zap.String("bucket", u.cfg.Bucket),
			zap.String("key", key),
			zap.Error(err),
		)

		return "", fmt.Errorf("presigning URL for s3://%s/%s: %w", u.cfg.Bucket, key, err)
	}

	return rewriteEndpoint(req.URL, u.cfg.PublicEndpoint), nil
}

func rewriteEndpoint(presignedURL, publicEndpoint string) string {
	u, err := url.Parse(presignedURL)
	if err != nil {
		return presignedURL
	}

	pub, err := url.Parse(publicEndpoint)
	if err != nil || pub.Scheme == "" || pub.Host == "" {
		return presignedURL
	}

	u.Scheme = pub.Scheme
	u.Host = pub.Host

	prefix := strings.TrimSuffix(pub.Path, "/")
	u.Path = prefix + u.Path

	return u.String()
}
