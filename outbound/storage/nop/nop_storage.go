package nop

import (
	"context"
	"errors"
	"io"
)

// Storage is a placeholder backend.
// TODO: replace with local or S3-compatible storage.
type Storage struct{}

func (Storage) Get(context.Context, string) (io.Reader, error) {
	return nil, errors.New("not found")
}

func (Storage) Put(context.Context, string, io.Reader) error {
	return nil
}

func (Storage) Exists(context.Context, string) (bool, error) {
	return false, nil
}
