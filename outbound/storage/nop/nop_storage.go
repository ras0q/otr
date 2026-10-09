package nop

import (
	"context"
	"errors"
	"io"

	"github.com/ras0q/otr/outbound/storage"
)

type Storage struct{}

var _ storage.Storage = Storage{}

func (Storage) Get(context.Context, string) (io.Reader, error) {
	return nil, errors.New("not found")
}

func (Storage) Put(context.Context, string, io.Reader) error {
	return nil
}

func (Storage) Exists(context.Context, string) (bool, error) {
	return false, nil
}
