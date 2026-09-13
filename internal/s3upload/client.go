package s3upload

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

const defaultPresignExpiry = 7 * 24 * time.Hour

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
		cfg.PresignExpiry = defaultPresignExpiry
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
