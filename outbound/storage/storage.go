package storage

import (
	"context"
	"io"
)

type Storage interface {
	Get(ctx context.Context, key string) (io.Reader, error)
	Put(ctx context.Context, key string, object io.Reader) error
	Exists(ctx context.Context, key string) (bool, error)
}
