package source

import (
	"context"
	"net/url"
)

type Source interface {
	GetLatestRelease(ctx context.Context, id Identity) (Release, error)
}

// Identity is owner/name on the one source wired into this process (e.g. cli/cli).
type Identity string

type Release struct {
	Version       Version
	RepositoryURL string
	Assets        []Asset
}

// Version is the version string
type Version string

type Asset struct {
	Name   string
	Size   int64
	Digest string
	URL    url.URL
}
