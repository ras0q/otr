package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ValidateKey(key Key) error {
	s := strings.TrimSpace(string(key))
	if s == "" {
		return errors.New("empty key")
	}
	if filepath.IsAbs(s) {
		return errors.New("absolute key")
	}
	clean := filepath.Clean(filepath.FromSlash(s))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return errors.New("invalid key")
	}
	return nil
}
