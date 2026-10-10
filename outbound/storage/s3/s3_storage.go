package s3

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3api "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ras0q/otr/outbound/storage"
	"golang.org/x/sync/singleflight"
)

const metaSHA1 = "otr-sha1"

// Storage implements storage.Storage against S3-compatible APIs.
type Storage struct {
	client *s3api.Client
	bucket string
	prefix string
	sf     singleflight.Group
}

var _ storage.Storage = (*Storage)(nil)

func NewStorage(ctx context.Context, cfg Config) (*Storage, error) {
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, errors.New("s3 bucket is required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		return nil, errors.New("s3 region is required")
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if cfg.AccessKeyID != "" || cfg.SecretKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3api.NewFromConfig(awsCfg, func(o *s3api.Options) {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &Storage{
		client: client,
		bucket: bucket,
		prefix: strings.Trim(cfg.Prefix, "/"),
	}, nil
}

func (s *Storage) Head(ctx context.Context, key storage.Key) (storage.Info, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.Info{}, err
	}
	objectKey := s.objectKey(key)

	out, err := s.client.HeadObject(ctx, &s3api.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if isNotFound(err) {
			return storage.Info{}, storage.ErrNotFound
		}
		return storage.Info{}, fmt.Errorf("s3 head object: %w", err)
	}

	sha1sum := out.Metadata[metaSHA1]
	if sha1sum == "" {
		return storage.Info{}, fmt.Errorf("missing %q metadata for key %q", metaSHA1, key)
	}
	return storage.Info{
		Size: aws.ToInt64(out.ContentLength),
		SHA1: sha1sum,
	}, nil
}

func (s *Storage) Open(ctx context.Context, key storage.Key) (io.ReadCloser, storage.Info, error) {
	info, err := s.Head(ctx, key)
	if err != nil {
		return nil, storage.Info{}, err
	}

	objectKey := s.objectKey(key)
	out, err := s.client.GetObject(ctx, &s3api.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, storage.Info{}, storage.ErrNotFound
		}
		return nil, storage.Info{}, fmt.Errorf("s3 get object: %w", err)
	}
	return out.Body, info, nil
}

func (s *Storage) PopulateIfAbsent(ctx context.Context, key storage.Key, fill storage.FillFunc) (storage.Info, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.Info{}, err
	}

	if info, err := s.Head(ctx, key); err == nil {
		return info, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return storage.Info{}, err
	}

	v, err, _ := s.sf.Do(string(key), func() (any, error) {
		if info, err := s.Head(ctx, key); err == nil {
			return info, nil
		} else if !errors.Is(err, storage.ErrNotFound) {
			return storage.Info{}, err
		}
		return s.materialize(ctx, key, fill)
	})
	if err != nil {
		return storage.Info{}, err
	}
	return v.(storage.Info), nil
}

func (s *Storage) materialize(ctx context.Context, key storage.Key, fill storage.FillFunc) (storage.Info, error) {
	tmp, err := os.CreateTemp("", "otr-s3-put-*")
	if err != nil {
		return storage.Info{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	hasher := sha1.New()
	tracked := io.MultiWriter(tmp, hasher)
	if err := fill(ctx, tracked); err != nil {
		_ = tmp.Close()
		cleanup()
		return storage.Info{}, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return storage.Info{}, fmt.Errorf("close temp file: %w", err)
	}

	stat, err := os.Stat(tmpPath)
	if err != nil {
		cleanup()
		return storage.Info{}, fmt.Errorf("stat temp file: %w", err)
	}

	sha1sum := hex.EncodeToString(hasher.Sum(nil))
	objectKey := s.objectKey(key)

	upload, err := os.Open(tmpPath)
	if err != nil {
		cleanup()
		return storage.Info{}, fmt.Errorf("open temp file: %w", err)
	}
	defer func() {
		_ = upload.Close()
		cleanup()
	}()

	_, err = s.client.PutObject(ctx, &s3api.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(objectKey),
		Body:          upload,
		ContentLength: aws.Int64(stat.Size()),
		ContentType:   aws.String("application/octet-stream"),
		Metadata: map[string]string{
			metaSHA1: sha1sum,
		},
	})
	if err != nil {
		return storage.Info{}, fmt.Errorf("s3 put object: %w", err)
	}

	return storage.Info{Size: stat.Size(), SHA1: sha1sum}, nil
}

func (s *Storage) objectKey(key storage.Key) string {
	k := strings.TrimPrefix(string(key), "/")
	if s.prefix == "" {
		return k
	}
	return s.prefix + "/" + k
}

func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	var notFound *types.NotFound
	if errors.As(err, &noSuchKey) || errors.As(err, &notFound) {
		return true
	}
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == 404
}
