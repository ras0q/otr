package local_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ras0q/otr/outbound/storage"
	"github.com/ras0q/otr/outbound/storage/local"
)

func TestPopulateHeadOpen(t *testing.T) {
	store, err := local.NewStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := storage.Key("tarballs/@otr/example--tool/1.0.0.tgz")

	info, err := store.PopulateIfAbsent(ctx, key, func(_ context.Context, w io.Writer) error {
		_, err := io.Copy(w, strings.NewReader("tarball-bytes"))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != int64(len("tarball-bytes")) {
		t.Fatalf("size: got %d", info.Size)
	}
	if len(info.SHA1) != 40 {
		t.Fatalf("sha1: %q", info.SHA1)
	}

	info2, err := store.Head(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if info2.SHA1 != info.SHA1 {
		t.Fatalf("head sha1 mismatch: %q vs %q", info2.SHA1, info.SHA1)
	}

	rc, openInfo, err := store.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "tarball-bytes" {
		t.Fatalf("body: %q", body)
	}
	if openInfo.SHA1 != info.SHA1 {
		t.Fatalf("open info sha1: %q", openInfo.SHA1)
	}

	_, err = store.Head(ctx, storage.Key("missing/object"))
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
