package app

import (
	"io"

	"github.com/aquaproj/aqua/v2/pkg/asset"
	"github.com/ras0q/otr/outbound/source"
)

// Identity is owner/name on the one source wired into this process.
type Identity = source.Identity

// Catalog is a versioned view of installable artifacts for one upstream identity.
type Catalog struct {
	Identity      Identity
	Version       source.Version
	RepositoryURL string
	Artifacts     []Artifact
}

// TODO: define originally
type Artifact asset.AssetInfo

// Target selects a platform-specific variant (os / arch / libc).
type Target struct {
	OS, Arch, Libc string
}

// ReleaseAsset is a versioned release file selected for a platform target.
type ReleaseAsset struct {
	Name string
	Size int64
	Body io.ReadCloser
}
