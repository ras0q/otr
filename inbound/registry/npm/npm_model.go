package npm

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ras0q/otr/app"
)

// JSON shapes follow:
// https://github.com/npm/registry/blob/main/docs/responses/package-metadata.md

type Dist struct {
	Shasum string `json:"shasum,omitempty"`
	// Integrity string `json:"integrity,omitempty"`
	Tarball string `json:"tarball"`
}

type Version struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// Deprecated           string            `json:"deprecated,omitempty"`
	// Dependencies         map[string]string `json:"dependencies,omitempty"`
	OptionalDependencies map[string]string `json:"optionalDependencies,omitempty"`
	// DevDependencies      map[string]string `json:"devDependencies,omitempty"`
	// BundleDependencies   []string          `json:"bundleDependencies,omitempty"`
	// PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
	Bin         map[string]string `json:"bin,omitempty"`
	Directories map[string]string `json:"directories,omitempty"`
	Dist        Dist              `json:"dist"`
	// Engines              map[string]string `json:"engines,omitempty"`
	HasShrinkwrap *bool `json:"_hasShrinkwrap,omitempty"`
	// HasInstallScript     bool              `json:"hasInstallScript,omitempty"`
	OS  []string `json:"os,omitempty"`
	CPU []string `json:"cpu,omitempty"`
}

type PackumentInstall struct {
	Name     string             `json:"name"`
	Modified string             `json:"modified"`
	DistTags map[string]string  `json:"dist-tags"`
	Versions map[string]Version `json:"versions"`
}

// type Human struct {
// 	Name  string `json:"name,omitempty"`
// 	Email string `json:"email,omitempty"`
// 	URL   string `json:"url,omitempty"`
// }

type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type VersionDocument struct {
	Version
	// Main           string            `json:"main,omitempty"`
	Description string      `json:"description,omitempty"`
	License     string      `json:"license,omitempty"`
	Repository  *Repository `json:"repository,omitempty"`
	// Readme         string      `json:"readme,omitempty"`
	ReadmeFilename string `json:"readmeFilename,omitempty"`
	ID             string `json:"_id,omitempty"`
	// NodeVersion    string            `json:"_nodeVersion,omitempty"`
	// NpmVersion     string            `json:"_npmVersion,omitempty"`
	// NpmUser        *Human            `json:"_npmUser,omitempty"`
	// Shasum         string            `json:"_shasum,omitempty"`
	// Maintainers    []Human           `json:"maintainers,omitempty"`
	// Scripts        map[string]string `json:"scripts,omitempty"`
}

type PackumentFull struct {
	ID          string                     `json:"_id"`
	Rev         string                     `json:"_rev,omitempty"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	License     string                     `json:"license,omitempty"`
	DistTags    map[string]string          `json:"dist-tags"`
	Versions    map[string]VersionDocument `json:"versions"`
	Time        map[string]string          `json:"time,omitempty"`
	Readme      string                     `json:"readme,omitempty"`
	Repository  *Repository                `json:"repository,omitempty"`
	// Maintainers []Human                    `json:"maintainers,omitempty"`
}

// PackumentInput carries resolved catalog data and registry presentation settings.
type PackumentInput struct {
	Scope        string
	BaseURL      string
	Catalog      app.Catalog
	Target       *app.Target
	Modified      string
	TarballShasum string
}

// NewPackumentInstall builds the install-v1 packument for a tool or platform package.
func NewPackumentInstall(packumentInput PackumentInput) PackumentInstall {
	name, catalogVersion := packumentNameAndVersion(packumentInput)
	return PackumentInstall{
		Name:     name,
		Modified: packumentInput.Modified,
		DistTags: map[string]string{"latest": catalogVersion},
		Versions: map[string]Version{catalogVersion: buildVersion(packumentInput, name, catalogVersion)},
	}
}

// NewPackumentFull builds the full metadata packument for a tool or platform package.
func NewPackumentFull(packumentInput PackumentInput) PackumentFull {
	name, catalogVersion := packumentNameAndVersion(packumentInput)
	repository := repositoryFromCatalog(packumentInput.Catalog)
	versionDocument := buildVersionDocument(packumentInput, name, catalogVersion, repository)
	packumentFull := PackumentFull{
		ID:         name,
		Rev:        "1",
		Name:       name,
		License:    "MIT",
		DistTags:   map[string]string{"latest": catalogVersion},
		Versions:   map[string]VersionDocument{catalogVersion: versionDocument},
		Time:       map[string]string{"modified": packumentInput.Modified, catalogVersion: packumentInput.Modified},
		Repository: repository,
	}
	if packumentInput.Target != nil {
		packumentFull.Description = "OTR platform package"
		return packumentFull
	}
	packumentFull.Description = "OTR tool package"
	packumentFull.Readme = "# OTR\n"
	return packumentFull
}

// NewVersionDocument builds a single version document for GET /{package}/{version}.
func NewVersionDocument(packumentInput PackumentInput, version string) VersionDocument {
	name, _ := packumentNameAndVersion(packumentInput)
	return buildVersionDocument(packumentInput, name, version, repositoryFromCatalog(packumentInput.Catalog))
}

func packumentNameAndVersion(packumentInput PackumentInput) (name, version string) {
	version = string(packumentInput.Catalog.Version)
	repositorySlug := strings.ReplaceAll(string(packumentInput.Catalog.Identity), "/", "--")
	if packumentInput.Target != nil {
		suffix := packumentInput.Target.OS + "-" + packumentInput.Target.Arch
		if packumentInput.Target.Libc != "" {
			suffix += "-" + packumentInput.Target.Libc
		}
		return packumentInput.Scope + "/" + repositorySlug + "--" + suffix, version
	}
	return packumentInput.Scope + "/" + repositorySlug, version
}

func buildVersion(packumentInput PackumentInput, name, version string) Version {
	hasShrinkwrap := false
	if packumentInput.Target != nil {
		return Version{
			Name:          name,
			Version:       version,
			OS:            []string{packumentInput.Target.OS},
			CPU:           []string{packumentInput.Target.Arch},
			Directories:   map[string]string{},
			Dist:          distFor(name, version, packumentInput.BaseURL, packumentInput.TarballShasum),
			HasShrinkwrap: &hasShrinkwrap,
		}
	}
	repositorySlug := strings.ReplaceAll(string(packumentInput.Catalog.Identity), "/", "--")
	optionalDependencies := make(map[string]string, len(packumentInput.Catalog.Artifacts))
	for _, artifact := range packumentInput.Catalog.Artifacts {
		platformTarget := &app.Target{OS: artifact.OS, Arch: artifact.Arch}
		optionalDependencies[packumentInput.Scope+"/"+repositorySlug+"--"+platformTarget.OS+"-"+platformTarget.Arch] = version
	}
	return Version{
		Name:    name,
		Version: version,
		Bin: map[string]string{
			"gh": "./bin/otr-launcher.mjs",
		},
		OptionalDependencies: optionalDependencies,
		Directories:          map[string]string{},
		Dist:                 distFor(name, version, packumentInput.BaseURL, packumentInput.TarballShasum),
		HasShrinkwrap:        &hasShrinkwrap,
	}
}

func buildVersionDocument(packumentInput PackumentInput, name, version string, repository *Repository) VersionDocument {
	return VersionDocument{
		Version:        buildVersion(packumentInput, name, version),
		Description:    fmt.Sprintf("OTR package for %s", name),
		License:        "MIT",
		ID:             name + "@" + version,
		ReadmeFilename: "README.md",
		Repository:     repository,
	}
}

func repositoryFromCatalog(catalog app.Catalog) *Repository {
	return &Repository{Type: "git", URL: catalog.RepositoryURL}
}

func distFor(name, version, baseURL, shasum string) Dist {
	return Dist{
		Tarball: tarballURL(baseURL, name, version),
		Shasum:  shasum,
	}
}

func tarballURL(baseURL, name, version string) string {
	normalizedBaseURL := strings.TrimSuffix(baseURL, "/")
	unscopedName := name
	if lastSlash := strings.LastIndex(name, "/"); lastSlash >= 0 {
		unscopedName = name[lastSlash+1:]
	}
	filename := fmt.Sprintf("%s-%s.tgz", unscopedName, version)
	return fmt.Sprintf("%s/%s/-/%s", normalizedBaseURL, url.PathEscape(name), filename)
}
