package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/inbound/registry/npm"
	"github.com/ras0q/otr/outbound/storage"
	"github.com/ras0q/otr/outbound/storage/local"
	s3store "github.com/ras0q/otr/outbound/storage/s3"
	"github.com/ras0q/otr/outbound/source/github"
)

// CLI holds otr server flags and environment-backed settings.
type CLI struct {
	Listen         string `help:"HTTP listen address." env:"OTR_LISTEN" default:":8080"`
	GitHubToken    string `name:"github-token" help:"GitHub API token for release resolution." env:"OTR_GITHUB_TOKEN"`
	PublicURL      string `name:"public-url" help:"Registry base URL for tarball and metadata links (e.g. https://npm.otr.dev)." env:"OTR_PUBLIC_URL"`
	NPMScope       string `name:"npm-scope" help:"npm scope for published package names (e.g. @otr)." env:"OTR_NPM_SCOPE" default:"@otr"`
	StorageBackend string `name:"storage-backend" help:"Storage backend for materialized npm tarballs." env:"OTR_STORAGE_BACKEND" default:"local" enum:"local,s3"`
	StorageDir     string `name:"storage-dir" help:"Local directory for materialized npm tarballs." env:"OTR_STORAGE_DIR" default:".otr/storage"`
	S3Bucket       string `name:"s3-bucket" help:"S3 bucket name." env:"OTR_S3_BUCKET"`
	S3Region       string `name:"s3-region" help:"S3 region." env:"OTR_S3_REGION"`
	S3Endpoint     string `name:"s3-endpoint" help:"S3-compatible endpoint URL (MinIO, R2, etc.)." env:"OTR_S3_ENDPOINT"`
	S3Prefix       string `name:"s3-prefix" help:"Optional key prefix inside the bucket." env:"OTR_S3_PREFIX"`
	S3PathStyle    bool   `name:"s3-path-style" help:"Use path-style S3 URLs." env:"OTR_S3_PATH_STYLE" default:"false"`
	S3AccessKeyID  string `name:"s3-access-key-id" help:"S3 access key (optional; default AWS credential chain)." env:"OTR_S3_ACCESS_KEY_ID"`
	S3SecretKey    string `name:"s3-secret-access-key" help:"S3 secret key." env:"OTR_S3_SECRET_ACCESS_KEY"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	ctx := context.Background()
	store, err := openStorage(ctx, cli)
	if err != nil {
		log.Fatal(err)
	}

	githubSource := github.NewSource(cli.GitHubToken)
	service := app.NewService(githubSource)
	registryHandler := npm.NewRegistry(service, store, npm.Config{
		PublicURL: cli.PublicURL,
		Scope:     cli.NPMScope,
	})

	log.Printf("otr npm registry listening on %s (storage: %s)", cli.Listen, cli.StorageBackend)
	if err := http.ListenAndServe(cli.Listen, registryHandler); err != nil {
		log.Fatal(err)
	}
}

func openStorage(ctx context.Context, cli CLI) (storage.Storage, error) {
	switch strings.ToLower(strings.TrimSpace(cli.StorageBackend)) {
	case "local":
		return local.NewStorage(cli.StorageDir)
	case "s3":
		return s3store.NewStorage(ctx, s3store.Config{
			Bucket:       cli.S3Bucket,
			Region:       cli.S3Region,
			Endpoint:     cli.S3Endpoint,
			Prefix:       cli.S3Prefix,
			UsePathStyle: cli.S3PathStyle,
			AccessKeyID:  cli.S3AccessKeyID,
			SecretKey:    cli.S3SecretKey,
		})
	default:
		return nil, fmt.Errorf("unknown storage backend %q", cli.StorageBackend)
	}
}
