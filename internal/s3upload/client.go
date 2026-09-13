package s3upload

import (
	"fmt"
	"net/url"
	"strings"
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

	// PublicBaseURL, when set, replaces presigned URLs with durable unsigned ones of the form PublicBaseURL/<key>.
	// Access control is delegated to whatever serves that base, such as CloudFront signed cookies. KeyPrefix
	// namespaces object keys in this mode so they fall under a signed-cookie policy resource.
	PublicBaseURL string
	KeyPrefix     string
}

type Client struct {
	cfg Config
	log *zap.Logger

	writer *s3.Client
	reader *s3.Client
}

func New(cfg Config, log *zap.Logger) (*Client, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3upload: S3_ENDPOINT, S3_BUCKET, S3_REGION, S3_ACCESS_KEY and S3_SECRET_KEY are required")
	}

	writer := newClient(cfg.Endpoint, cfg.Region, cfg.AccessKey, cfg.SecretKey)

	if cfg.PublicBaseURL != "" {
		base, err := url.Parse(cfg.PublicBaseURL)
		if err != nil || base.Scheme == "" || base.Host == "" {
			return nil, fmt.Errorf("s3upload: invalid public base URL %q", cfg.PublicBaseURL)
		}

		cfg.KeyPrefix = strings.Trim(cfg.KeyPrefix, "/")
		if cfg.KeyPrefix == "" {
			return nil, fmt.Errorf("s3upload: a key prefix is required when a public base URL is set")
		}

		return &Client{cfg: cfg, log: log, writer: writer}, nil
	}

	if cfg.ReadonlyAccessKey == "" || cfg.ReadonlySecretKey == "" {
		return nil, fmt.Errorf(
			"s3upload: S3_READONLY_ACCESS_KEY and S3_READONLY_SECRET_KEY are required unless a public base URL is set",
		)
	}

	if cfg.PresignExpiry <= 0 {
		cfg.PresignExpiry = defaultPresignExpiry
	}

	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}

	reader := newClient(cfg.PublicEndpoint, cfg.Region, cfg.ReadonlyAccessKey, cfg.ReadonlySecretKey)

	return &Client{cfg: cfg, log: log, writer: writer, reader: reader}, nil
}

func newClient(endpoint, region, accessKey, secretKey string) *s3.Client {
	return s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	})
}
