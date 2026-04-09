// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store implements Store using any S3-compatible API (AWS, Garage, Cloudflare R2, etc.).
// Path-style access is enabled by default for non-AWS endpoints.
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Store creates a new S3-compatible blob store.
//   - endpoint: custom S3 endpoint URL (empty = AWS default)
//   - region: AWS region (e.g. "us-east-1", "garage" for Garage)
//   - bucket: S3 bucket name
//   - prefix: optional key prefix (e.g. "modules/")
//   - accessKey, secretKey: S3 credentials
func NewS3Store(endpoint, region, bucket, prefix, accessKey, secretKey string) (*S3Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = true // required for Garage, R2, and most non-AWS S3
	})

	return &S3Store{client: client, bucket: bucket, prefix: prefix}, nil
}

func (s *S3Store) key(k string) string {
	if s.prefix != "" {
		return s.prefix + k
	}
	return k
}

// Put uploads a blob to S3.
func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
		Body:   bytes.NewReader(data),
	})
	return err
}

// Get retrieves a blob from S3. Returns ErrNotFound if the key doesn't exist.
func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}
