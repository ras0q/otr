package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ras0q/otr/outbound/storage"
)

// Storage stores objects as files under Root, keyed by slash-separated paths.
type Storage struct {
	Root string
}

var _ storage.Storage = Storage{}

func New(root string) (Storage, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Storage{}, fmt.Errorf("create storage root: %w", err)
	}
	return Storage{Root: root}, nil
}

func (s Storage) Get(_ context.Context, key string) (io.Reader, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not found: %w", err)
		}
		return nil, err
	}
	return f, nil
}

func (s Storage) Put(_ context.Context, key string, object io.Reader) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".put-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := io.Copy(tmp, object); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (s Storage) Exists(_ context.Context, key string) (bool, error) {
	path, err := s.resolve(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s Storage) resolve(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("empty key")
	}
	if filepath.IsAbs(key) {
		return "", errors.New("absolute key")
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("invalid key")
	}
	path := filepath.Join(s.Root, clean)
	rel, err := filepath.Rel(s.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("invalid key")
	}
	return path, nil
}
