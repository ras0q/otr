package storage

import (
	"context"
	"fmt"
	"io"
)

// OpenCached returns a stored object, calling build when the key is missing.
func OpenCached(ctx context.Context, backend Storage, key string, build func(context.Context, io.Writer) error) (io.ReadCloser, error) {
	ok, err := backend.Exists(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("storage exists: %w", err)
	}
	if ok {
		return openReader(ctx, backend, key)
	}

	pipeReader, pipeWriter := io.Pipe()
	buildDone := make(chan error, 1)
	go func() {
		err := build(ctx, pipeWriter)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			buildDone <- err
			return
		}
		buildDone <- pipeWriter.Close()
	}()

	if err := backend.Put(ctx, key, pipeReader); err != nil {
		_ = pipeReader.Close()
		return nil, fmt.Errorf("storage put: %w", err)
	}
	if err := <-buildDone; err != nil {
		return nil, err
	}
	return openReader(ctx, backend, key)
}

func openReader(ctx context.Context, backend Storage, key string) (io.ReadCloser, error) {
	objectReader, err := backend.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("storage get: %w", err)
	}
	readCloser, ok := objectReader.(io.ReadCloser)
	if ok {
		return readCloser, nil
	}
	return io.NopCloser(objectReader), nil
}
