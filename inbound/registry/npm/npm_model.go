package npm

// JSON shapes follow:
// https://github.com/npm/registry/blob/main/docs/responses/package-metadata.md

type Dist struct {
	Shasum    string `json:"shasum,omitempty"`
	Integrity string `json:"integrity,omitempty"`
	Tarball   string `json:"tarball"`
}

type Version struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Deprecated           string            `json:"deprecated,omitempty"`
	Dependencies         map[string]string `json:"dependencies,omitempty"`
	OptionalDependencies map[string]string `json:"optionalDependencies,omitempty"`
	DevDependencies      map[string]string `json:"devDependencies,omitempty"`
	BundleDependencies   []string          `json:"bundleDependencies,omitempty"`
	PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
	Bin                  map[string]string `json:"bin,omitempty"`
	Directories          map[string]string `json:"directories,omitempty"`
	Dist                 Dist              `json:"dist"`
	Engines              map[string]string `json:"engines,omitempty"`
	HasShrinkwrap        *bool             `json:"_hasShrinkwrap,omitempty"`
	HasInstallScript     bool              `json:"hasInstallScript,omitempty"`
	OS                   []string          `json:"os,omitempty"`
	CPU                  []string          `json:"cpu,omitempty"`
}

type PackumentInstall struct {
	Name     string             `json:"name"`
	Modified string             `json:"modified"`
	DistTags map[string]string  `json:"dist-tags"`
	Versions map[string]Version `json:"versions"`
}

type Human struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type VersionDocument struct {
	Version
	Main           string            `json:"main,omitempty"`
	Description    string            `json:"description,omitempty"`
	License        string            `json:"license,omitempty"`
	Repository     *Repository       `json:"repository,omitempty"`
	Readme         string            `json:"readme,omitempty"`
	ReadmeFilename string            `json:"readmeFilename,omitempty"`
	ID             string            `json:"_id,omitempty"`
	NodeVersion    string            `json:"_nodeVersion,omitempty"`
	NpmVersion     string            `json:"_npmVersion,omitempty"`
	NpmUser        *Human            `json:"_npmUser,omitempty"`
	Shasum         string            `json:"_shasum,omitempty"`
	Maintainers    []Human           `json:"maintainers,omitempty"`
	Scripts        map[string]string `json:"scripts,omitempty"`
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
	Maintainers []Human                    `json:"maintainers,omitempty"`
}
