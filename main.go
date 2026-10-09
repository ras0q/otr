package main

import (
	"log"
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/inbound/registry/npm"
	"github.com/ras0q/otr/outbound/source/github"
	"github.com/ras0q/otr/outbound/storage/local"
)

// CLI holds otr server flags and environment-backed settings.
type CLI struct {
	Listen      string `help:"HTTP listen address." env:"OTR_LISTEN" default:":8080"`
	GitHubToken string `name:"github-token" help:"GitHub API token for release resolution." env:"OTR_GITHUB_TOKEN"`
	PublicURL   string `name:"public-url" help:"Registry base URL for tarball and metadata links (e.g. https://npm.otr.dev)." env:"OTR_PUBLIC_URL"`
	NPMScope    string `name:"npm-scope" help:"npm scope for published package names (e.g. @otr)." env:"OTR_NPM_SCOPE" default:"@otr"`
	StorageDir  string `name:"storage-dir" help:"Local directory for materialized npm tarballs." env:"OTR_STORAGE_DIR" default:".otr/storage"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	localStorage, err := local.New(cli.StorageDir)
	if err != nil {
		log.Fatal(err)
	}

	githubSource := github.NewSource(cli.GitHubToken)
	service := app.NewService(githubSource)
	registryHandler := npm.NewRegistry(service, localStorage, npm.Config{
		PublicURL: cli.PublicURL,
		Scope:     cli.NPMScope,
	})

	log.Printf("otr npm registry listening on %s", cli.Listen)
	if err := http.ListenAndServe(cli.Listen, registryHandler); err != nil {
		log.Fatal(err)
	}
}
