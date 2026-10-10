package s3_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ras0q/otr/outbound/storage"
	s3store "github.com/ras0q/otr/outbound/storage/s3"
)

func TestS3PopulateHeadOpen(t *testing.T) {
	endpoint := os.Getenv("OTR_S3_TEST_ENDPOINT")
	bucket := os.Getenv("OTR_S3_TEST_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("set OTR_S3_TEST_ENDPOINT and OTR_S3_TEST_BUCKET for S3 integration test")
	}

	ctx := context.Background()
	store, err := s3store.NewStorage(ctx, s3store.Config{
		Bucket:       bucket,
		Region:       envOr("OTR_S3_TEST_REGION", "us-east-1"),
		Endpoint:     endpoint,
		UsePathStyle: os.Getenv("OTR_S3_TEST_PATH_STYLE") == "1",
		AccessKeyID:  os.Getenv("OTR_S3_TEST_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("OTR_S3_TEST_SECRET_ACCESS_KEY"),
		Prefix:       "test/" + t.Name(),
	})
	if err != nil {
		t.Fatal(err)
	}

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

	info2, err := store.Head(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if info2.SHA1 != info.SHA1 {
		t.Fatalf("sha1 mismatch")
	}

	rc, _, err := store.Open(ctx, key)
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
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
