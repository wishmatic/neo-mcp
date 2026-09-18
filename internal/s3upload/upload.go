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

const presignExpiry = 7 * 24 * time.Hour

// PublicKeyPrefix is the object key prefix Garagefront serves without access checks.
const PublicKeyPrefix = "i/public"

// NSFWKeySegment namespaces NSFW objects within their namespace so they can be served or restricted separately.
const NSFWKeySegment = "nsfw"

func (u *Client) UploadFile(ctx context.Context, data []byte, contentType string, public, nsfw bool) (string, error) {
	key := u.objectKey(public, nsfw, fileExtension(contentType))

	u.log.Info("uploading image",
		zap.String("key", key),
		zap.String("content_type", contentType),
		zap.Int("bytes", len(data)),
	)

	_, err := u.writer.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		u.log.Error("s3 put object failed",
			zap.String("bucket", u.cfg.Bucket),
			zap.String("key", key),
			zap.Error(err),
		)

		return "", fmt.Errorf("uploading image to s3://%s/%s: %w", u.cfg.Bucket, key, err)
	}

	if u.cfg.PublicBaseURL != "" {
		return publicObjectURL(u.cfg.PublicBaseURL, key), nil
	}

	return u.presignedURL(ctx, key)
}

func fileExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "jpg"
	case "image/jxl":
		return "jxl"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

func (u *Client) objectKey(public, nsfw bool, ext string) string {
	var parts []string

	switch {
	case public:
		parts = append(parts, PublicKeyPrefix)
	case u.cfg.PublicBaseURL != "" && u.cfg.KeyPrefix != "":
		parts = append(parts, u.cfg.KeyPrefix)
	}

	if nsfw {
		parts = append(parts, NSFWKeySegment)
	}

	name := fmt.Sprintf("%s/%s.%s", time.Now().UTC().Format("2006-01"), uuid.NewString(), ext)

	return strings.Join(append(parts, name), "/")
}

// IsPublicURL reports whether rawURL is an unsigned URL for an object under PublicKeyPrefix on PublicBaseURL.
func (u *Client) IsPublicURL(rawURL string) bool {
	if u.cfg.PublicBaseURL == "" {
		return false
	}

	base, err := url.Parse(u.cfg.PublicBaseURL)
	if err != nil || base.Host == "" {
		return false
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	if !strings.EqualFold(target.Host, base.Host) {
		return false
	}

	prefix := strings.TrimSuffix(base.Path, "/") + "/" + PublicKeyPrefix + "/"

	return strings.HasPrefix(target.Path, prefix)
}

func publicObjectURL(base, key string) string {
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimSuffix(base, "/") + "/" + key
	}

	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + key

	return u.String()
}

func (u *Client) presignedURL(ctx context.Context, key string) (string, error) {
	presigner := s3.NewPresignClient(u.reader)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(presignExpiry))
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
