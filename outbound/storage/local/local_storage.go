package local

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ras0q/otr/outbound/storage"
	"golang.org/x/sync/singleflight"
)

type meta struct {
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

// Storage persists files under Root with a JSON sidecar for metadata.
type Storage struct {
	Root string
	sf   singleflight.Group
}

var _ storage.Storage = (*Storage)(nil)

func NewStorage(root string) (*Storage, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &Storage{Root: root}, nil
}

func (s *Storage) Head(ctx context.Context, key storage.Key) (storage.Info, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.Info{}, err
	}
	objectPath, err := s.resolve(key)
	if err != nil {
		return storage.Info{}, err
	}
	if _, err := os.Stat(objectPath); err != nil {
		if os.IsNotExist(err) {
			return storage.Info{}, storage.ErrNotFound
		}
		return storage.Info{}, err
	}
	objectMeta, err := s.readMeta(key)
	if err != nil {
		return storage.Info{}, err
	}
	return infoFromMeta(objectMeta), nil
}

func (s *Storage) Open(ctx context.Context, key storage.Key) (io.ReadCloser, storage.Info, error) {
	info, err := s.Head(ctx, key)
	if err != nil {
		return nil, storage.Info{}, err
	}
	objectPath, err := s.resolve(key)
	if err != nil {
		return nil, storage.Info{}, err
	}
	f, err := os.Open(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.Info{}, storage.ErrNotFound
		}
		return nil, storage.Info{}, err
	}
	return f, info, nil
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
	objectPath, err := s.resolve(key)
	if err != nil {
		return storage.Info{}, err
	}
	metaPath, err := s.resolveMeta(key)
	if err != nil {
		return storage.Info{}, err
	}

	if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		return storage.Info{}, fmt.Errorf("create parent dir: %w", err)
	}

	tmpObject, err := os.CreateTemp(filepath.Dir(objectPath), ".put-*")
	if err != nil {
		return storage.Info{}, fmt.Errorf("create temp object: %w", err)
	}
	tmpObjectPath := tmpObject.Name()
	cleanupObject := func() { _ = os.Remove(tmpObjectPath) }

	hasher := sha1.New()
	tracked := io.MultiWriter(tmpObject, hasher)
	if err := fill(ctx, tracked); err != nil {
		_ = tmpObject.Close()
		cleanupObject()
		return storage.Info{}, err
	}
	if err := tmpObject.Close(); err != nil {
		cleanupObject()
		return storage.Info{}, fmt.Errorf("close temp object: %w", err)
	}

	stat, err := os.Stat(tmpObjectPath)
	if err != nil {
		cleanupObject()
		return storage.Info{}, fmt.Errorf("stat temp object: %w", err)
	}

	objectMeta := meta{
		SHA1: hex.EncodeToString(hasher.Sum(nil)),
		Size: stat.Size(),
	}
	metaBytes, err := json.Marshal(objectMeta)
	if err != nil {
		cleanupObject()
		return storage.Info{}, fmt.Errorf("encode meta: %w", err)
	}

	tmpMeta, err := os.CreateTemp(filepath.Dir(metaPath), ".meta-*")
	if err != nil {
		cleanupObject()
		return storage.Info{}, fmt.Errorf("create temp meta: %w", err)
	}
	tmpMetaPath := tmpMeta.Name()
	cleanupMeta := func() { _ = os.Remove(tmpMetaPath) }
	if _, err := tmpMeta.Write(metaBytes); err != nil {
		_ = tmpMeta.Close()
		cleanupObject()
		cleanupMeta()
		return storage.Info{}, fmt.Errorf("write temp meta: %w", err)
	}
	if err := tmpMeta.Close(); err != nil {
		cleanupObject()
		cleanupMeta()
		return storage.Info{}, fmt.Errorf("close temp meta: %w", err)
	}

	if err := os.Rename(tmpObjectPath, objectPath); err != nil {
		cleanupObject()
		cleanupMeta()
		return storage.Info{}, fmt.Errorf("commit object: %w", err)
	}
	if err := os.Rename(tmpMetaPath, metaPath); err != nil {
		_ = os.Remove(objectPath)
		cleanupMeta()
		return storage.Info{}, fmt.Errorf("commit meta: %w", err)
	}

	return infoFromMeta(objectMeta), nil
}

func (s *Storage) readMeta(key storage.Key) (meta, error) {
	metaPath, err := s.resolveMeta(key)
	if err != nil {
		return meta{}, err
	}
	body, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return meta{}, storage.ErrNotFound
		}
		return meta{}, err
	}
	var objectMeta meta
	if err := json.Unmarshal(body, &objectMeta); err != nil {
		return meta{}, fmt.Errorf("decode meta: %w", err)
	}
	if objectMeta.SHA1 == "" {
		return meta{}, fmt.Errorf("missing sha1 in meta for %q", key)
	}
	return objectMeta, nil
}

func (s *Storage) resolve(key storage.Key) (string, error) {
	if err := storage.ValidateKey(key); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(string(key)))
	path := filepath.Join(s.Root, clean)
	rel, err := filepath.Rel(s.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("invalid key")
	}
	return path, nil
}

func (s *Storage) resolveMeta(key storage.Key) (string, error) {
	return s.resolve(storage.Key(string(key) + ".meta"))
}

func infoFromMeta(m meta) storage.Info {
	return storage.Info{Size: m.Size, SHA1: m.SHA1}
}
