package s3upload

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Config struct {
	Endpoint          string
	PublicEndpoint    string
	Bucket            string
	Region            string
	AccessKey         string
	SecretKey         string
	ReadonlyAccessKey string
	ReadonlySecretKey string
	PresignExpiry     time.Duration
}

type Uploader struct {
	cfg Config
	log *zap.Logger

	writer *s3.Client
	reader *s3.Client
}

func New(cfg Config, log *zap.Logger) (*Uploader, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.Region == "" ||
		cfg.AccessKey == "" || cfg.SecretKey == "" ||
		cfg.ReadonlyAccessKey == "" || cfg.ReadonlySecretKey == "" {
		return nil, fmt.Errorf("s3upload: all S3_* config fields are required")
	}
	if cfg.PresignExpiry <= 0 {
		cfg.PresignExpiry = 7 * 24 * time.Hour
	}
	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}

	writer := newClient(cfg.Endpoint, cfg.Region, cfg.AccessKey, cfg.SecretKey)
	reader := newClient(cfg.PublicEndpoint, cfg.Region, cfg.ReadonlyAccessKey, cfg.ReadonlySecretKey)

	return &Uploader{cfg: cfg, log: log, writer: writer, reader: reader}, nil
}

func newClient(endpoint, region, accessKey, secretKey string) *s3.Client {
	return s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	})
}

func (u *Uploader) UploadImage(ctx context.Context, data []byte) (string, error) {
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
