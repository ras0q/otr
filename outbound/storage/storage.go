package storage

import (
	"context"
	"errors"
	"io"
)

var ErrNotFound = errors.New("not found")

// Key is a slash-separated object path (POSIX), e.g. tarballs/@otr/foo/1.0.0.tgz.
type Key string

type Info struct {
	Size int64
	SHA1 string // npm dist.shasum (40 lowercase hex digits)
}

type FillFunc func(ctx context.Context, w io.Writer) error

type Storage interface {
	Head(ctx context.Context, key Key) (Info, error)
	Open(ctx context.Context, key Key) (io.ReadCloser, Info, error)
	PopulateIfAbsent(ctx context.Context, key Key, fill FillFunc) (Info, error)
}
