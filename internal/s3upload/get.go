package s3upload

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GetObject reads an object through the internal endpoint using the writer credentials. Reads must target the
// internal endpoint rather than the public one, and Garagefront mode has no reader client to fall back on.
func (u *Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	out, err := u.writer.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s: %w", u.cfg.Bucket, key, err)
	}

	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3://%s/%s: %w", u.cfg.Bucket, key, err)
	}

	return data, nil
}
