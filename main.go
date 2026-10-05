package main

import (
	"log"
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/inbound/registry/npm"
	"github.com/ras0q/otr/outbound/source/github"
	"github.com/ras0q/otr/outbound/storage/nop"
)

// CLI holds otr server flags and environment-backed settings.
type CLI struct {
	Listen      string `help:"HTTP listen address." env:"OTR_LISTEN" default:":8080"`
	GitHubToken string `name:"github-token" help:"GitHub API token for release resolution." env:"OTR_GITHUB_TOKEN"`
	PublicURL   string `name:"public-url" help:"Registry base URL for tarball and metadata links (e.g. https://npm.otr.dev)." env:"OTR_PUBLIC_URL"`
	NPMScope    string `name:"npm-scope" help:"npm scope for published package names (e.g. @otr)." env:"OTR_NPM_SCOPE" default:"@otr"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	// TODO: OTR_STORAGE (local/S3) instead of nop.
	source := github.NewSource(cli.GitHubToken)
	catalog := app.NewResolver(source, nop.Storage{})
	handler := npm.NewRegistry(catalog, npm.Config{
		PublicURL: cli.PublicURL,
		Scope:     cli.NPMScope,
	})

	log.Printf("otr npm registry listening on %s", cli.Listen)
	if err := http.ListenAndServe(cli.Listen, handler); err != nil {
		log.Fatal(err)
	}
}
