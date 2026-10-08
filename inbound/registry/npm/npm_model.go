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
	Scope    string
	BaseURL  string
	Catalog  app.Catalog
	Target   *app.Target
	Modified string
}

// NewPackumentInstall builds the install-v1 packument for a tool or platform package.
func NewPackumentInstall(in PackumentInput) PackumentInstall {
	name, ver := packumentNameAndVersion(in)
	return PackumentInstall{
		Name:     name,
		Modified: in.Modified,
		DistTags: map[string]string{"latest": ver},
		Versions: map[string]Version{ver: buildVersion(in, name, ver)},
	}
}

// NewPackumentFull builds the full metadata packument for a tool or platform package.
func NewPackumentFull(in PackumentInput) PackumentFull {
	name, ver := packumentNameAndVersion(in)
	repo := repositoryFromCatalog(in.Catalog)
	doc := buildVersionDocument(in, name, ver, repo)
	out := PackumentFull{
		ID:         name,
		Rev:        "1",
		Name:       name,
		License:    "MIT",
		DistTags:   map[string]string{"latest": ver},
		Versions:   map[string]VersionDocument{ver: doc},
		Time:       map[string]string{"modified": in.Modified, ver: in.Modified},
		Repository: repo,
	}
	if in.Target != nil {
		out.Description = "OTR platform package"
		return out
	}
	out.Description = "OTR tool package"
	out.Readme = "# OTR\n"
	return out
}

// NewVersionDocument builds a single version document for GET /{package}/{version}.
func NewVersionDocument(in PackumentInput, version string) VersionDocument {
	name, _ := packumentNameAndVersion(in)
	return buildVersionDocument(in, name, version, repositoryFromCatalog(in.Catalog))
}

func packumentNameAndVersion(in PackumentInput) (name, version string) {
	version = string(in.Catalog.Version)
	repoBase := strings.ReplaceAll(string(in.Catalog.Identity), "/", "--")
	if in.Target != nil {
		suffix := in.Target.OS + "-" + in.Target.Arch
		if in.Target.Libc != "" {
			suffix += "-" + in.Target.Libc
		}
		return in.Scope + "/" + repoBase + "--" + suffix, version
	}
	return in.Scope + "/" + repoBase, version
}

func buildVersion(in PackumentInput, name, version string) Version {
	hasShrinkwrap := false
	if in.Target != nil {
		return Version{
			Name:          name,
			Version:       version,
			OS:            []string{in.Target.OS},
			CPU:           []string{in.Target.Arch},
			Directories:   map[string]string{},
			Dist:          distFor(name, version, in.BaseURL),
			HasShrinkwrap: &hasShrinkwrap,
		}
	}
	repoBase := strings.ReplaceAll(string(in.Catalog.Identity), "/", "--")
	optionalDependencies := make(map[string]string, len(in.Catalog.Artifacts))
	for _, a := range in.Catalog.Artifacts {
		t := &app.Target{OS: a.OS, Arch: a.Arch}
		optionalDependencies[in.Scope+"/"+repoBase+"--"+t.OS+"-"+t.Arch] = version
	}
	return Version{
		Name:    name,
		Version: version,
		Bin: map[string]string{
			"gh": "./bin/otr-launcher.mjs",
		},
		OptionalDependencies: optionalDependencies,
		Directories:          map[string]string{},
		Dist:                 distFor(name, version, in.BaseURL),
		HasShrinkwrap:        &hasShrinkwrap,
	}
}

func buildVersionDocument(in PackumentInput, name, version string, repo *Repository) VersionDocument {
	return VersionDocument{
		Version:        buildVersion(in, name, version),
		Description:    fmt.Sprintf("OTR package for %s", name),
		License:        "MIT",
		ID:             name + "@" + version,
		ReadmeFilename: "README.md",
		Repository:     repo,
	}
}

func repositoryFromCatalog(catalog app.Catalog) *Repository {
	return &Repository{Type: "git", URL: catalog.RepositoryURL}
}

func distFor(name, version, baseURL string) Dist {
	return Dist{
		Tarball: tarballURL(baseURL, name, version),
		Shasum:  "0000000000000000000000000000000000000000", // TODO: real tarball shasum once packages are materialized.
	}
}

func tarballURL(baseURL, name, version string) string {
	base := strings.TrimSuffix(baseURL, "/")
	short := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		short = name[i+1:]
	}
	filename := fmt.Sprintf("%s-%s.tgz", short, version)
	return fmt.Sprintf("%s/%s/-/%s", base, url.PathEscape(name), filename)
}
