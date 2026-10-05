package npm

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/inbound/registry/reghttp"
)

type Config struct {
	PublicURL string
	Scope     string
}

type Registry struct {
	http.ServeMux
	resolver  app.Resolver
	publicURL string
	scope     string
}

var _ http.Handler = (*Registry)(nil)

func NewRegistry(catalog app.Resolver, cfg Config) *Registry {
	scope := strings.TrimSpace(cfg.Scope)
	if scope == "" {
		scope = "@otr"
	} else if !strings.HasPrefix(scope, "@") {
		scope = "@" + scope
	}

	r := &Registry{
		resolver:  catalog,
		publicURL: strings.TrimSuffix(strings.TrimSpace(cfg.PublicURL), "/"),
		scope:     scope,
	}

	r.Handle("GET /{package}", reghttp.Handle(r.GetPackage))
	r.Handle("GET /{package}/{version}", reghttp.Handle(r.GetPackageWithVersion))
	r.Handle("GET /{package}/-/{filename}", reghttp.Handle(r.DownloadPackage))

	return r
}

// GET /{package}
func (r *Registry) GetPackage(w http.ResponseWriter, req *http.Request) error {
	install := false
	for _, part := range strings.Split(req.Header.Get("Accept"), ",") {
		media := strings.TrimSpace(strings.Split(part, ";")[0])
		if strings.EqualFold(media, "application/vnd.npm.install-v1+json") {
			install = true
			break
		}
	}

	transportName, err := url.PathUnescape(req.PathValue("package"))
	if err != nil {
		return reghttp.BadRequest("decode package name: " + err.Error())
	}
	if transportName == "" {
		return reghttp.BadRequest("invalid package name")
	}

	id, target, err := parsePackageName(transportName)
	if err != nil {
		return reghttp.BadRequest(err.Error())
	}

	catalog, err := r.resolver.Resolve(req.Context(), id)
	if err != nil {
		return fmt.Errorf("resolve catalog: %w", err)
	}

	base := r.publicURL
	if base == "" {
		if req.Host == "" {
			base = "http://127.0.0.1"
		} else {
			base = "http://" + req.Host
		}
	}

	catalogVersion := string(catalog.Version)
	modified := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	repoBase := strings.ReplaceAll(string(id), "/", "--")
	repo := &Repository{Type: "git", URL: catalog.RepositoryURL}

	if target != nil {
		suffix := target.OS + "-" + target.Arch
		if target.Libc != "" {
			suffix += "-" + target.Libc
		}
		name := r.scope + "/" + repoBase + "--" + suffix
		hasShrinkwrap := false
		platformVer := Version{
			Name:          name,
			Version:       catalogVersion,
			OS:            []string{target.OS},
			CPU:           []string{target.Arch},
			Directories:   map[string]string{},
			Dist:          Dist{Tarball: tarballURL(base, name, catalogVersion), Shasum: "0000000000000000000000000000000000000000"}, // TODO: real tarball shasum once packages are materialized.
			HasShrinkwrap: &hasShrinkwrap,
		}
		if install {
			return reghttp.WriteJSON(w, "application/vnd.npm.install-v1+json", PackumentInstall{
				Name:     name,
				Modified: modified,
				DistTags: map[string]string{"latest": catalogVersion},
				Versions: map[string]Version{catalogVersion: platformVer},
			})
		}

		return reghttp.WriteJSON(w, "application/json", PackumentFull{
			ID:          name,
			Rev:         "1",
			Name:        name,
			Description: "OTR platform package",
			License:     "MIT",
			DistTags:    map[string]string{"latest": catalogVersion},
			Versions: map[string]VersionDocument{catalogVersion: {
				Version:        platformVer,
				Description:    "OTR package",
				License:        "MIT",
				ID:             name + "@" + catalogVersion,
				Readme:         "# OTR\n",
				ReadmeFilename: "README.md",
				Repository:     repo,
			}},
			Time:       map[string]string{"modified": modified, catalogVersion: modified},
			Repository: repo,
		})
	}

	toolName := r.scope + "/" + repoBase
	optionalDependencies := make(map[string]string, len(catalog.Artifacts))
	for _, a := range catalog.Artifacts {
		t := &app.Target{OS: a.OS, Arch: a.Arch}
		optionalDependencies[r.scope+"/"+repoBase+"--"+t.OS+"-"+t.Arch] = catalogVersion
	}

	hasShrinkwrap := false
	toolVersion := Version{
		Name:    toolName,
		Version: catalogVersion,
		Bin: map[string]string{
			"gh": "./bin/otr-launcher.mjs",
		},
		Dependencies: map[string]string{
			"@open-tool-registry/runtime": "1.0.0",
		},
		OptionalDependencies: optionalDependencies,
		Directories:          map[string]string{},
		Dist: Dist{
			Tarball: tarballURL(base, toolName, catalogVersion),
			Shasum:  "0000000000000000000000000000000000000000", // TODO: real tarball shasum once packages are materialized.
		},
		HasShrinkwrap: &hasShrinkwrap,
	}

	if install {
		return reghttp.WriteJSON(w, "application/vnd.npm.install-v1+json", PackumentInstall{
			Name:     toolName,
			Modified: modified,
			DistTags: map[string]string{"latest": catalogVersion},
			Versions: map[string]Version{catalogVersion: toolVersion},
		})
	}

	return reghttp.WriteJSON(w, "application/json", PackumentFull{
		ID:          toolName,
		Rev:         "1",
		Name:        toolName,
		Description: "OTR tool package",
		License:     "MIT",
		DistTags:    map[string]string{"latest": catalogVersion},
		Versions: map[string]VersionDocument{catalogVersion: {
			Version:        toolVersion,
			Description:    "OTR package",
			License:        "MIT",
			ID:             toolName + "@" + catalogVersion,
			Readme:         "# OTR\n",
			ReadmeFilename: "README.md",
			Repository:     repo,
		}},
		Time:       map[string]string{"modified": modified, catalogVersion: modified},
		Readme:     "# OTR\n",
		Repository: repo,
	})
}

// GET /{package}/{version}
func (r *Registry) GetPackageWithVersion(w http.ResponseWriter, req *http.Request) error {
	transportName, err := url.PathUnescape(req.PathValue("package"))
	if err != nil {
		return reghttp.BadRequest("decode package name: " + err.Error())
	}
	if transportName == "" {
		return reghttp.BadRequest("invalid package name")
	}

	id, target, err := parsePackageName(transportName)
	if err != nil {
		return reghttp.BadRequest(err.Error())
	}

	catalog, err := r.resolver.Resolve(req.Context(), id)
	if err != nil {
		return fmt.Errorf("resolve catalog: %w", err)
	}

	ver := req.PathValue("version")
	if ver == "latest" {
		ver = string(catalog.Version)
	} else if ver != string(catalog.Version) {
		return reghttp.StatusError(http.StatusNotFound, "version not found")
	}

	base := r.publicURL
	if base == "" {
		if req.Host == "" {
			base = "http://127.0.0.1"
		} else {
			base = "http://" + req.Host
		}
	}

	repoBase := strings.ReplaceAll(string(id), "/", "--")
	repo := &Repository{Type: "git", URL: catalog.RepositoryURL}

	if target != nil {
		suffix := target.OS + "-" + target.Arch
		if target.Libc != "" {
			suffix += "-" + target.Libc
		}
		name := r.scope + "/" + repoBase + "--" + suffix
		hasShrinkwrap := false
		v := Version{
			Name:          name,
			Version:       ver,
			OS:            []string{target.OS},
			CPU:           []string{target.Arch},
			Directories:   map[string]string{},
			Dist:          Dist{Tarball: tarballURL(base, name, ver), Shasum: "0000000000000000000000000000000000000000"}, // TODO: real tarball shasum once packages are materialized.
			HasShrinkwrap: &hasShrinkwrap,
		}

		return reghttp.WriteJSON(w, "application/json", VersionDocument{
			Version:        v,
			Description:    "OTR package",
			License:        "MIT",
			ID:             name + "@" + ver,
			Readme:         "# OTR\n",
			ReadmeFilename: "README.md",
			Repository:     repo,
		})
	}

	toolName := r.scope + "/" + repoBase
	optionalDependencies := make(map[string]string, len(catalog.Artifacts))
	for _, a := range catalog.Artifacts {
		t := &app.Target{OS: a.OS, Arch: a.Arch}
		optionalDependencies[r.scope+"/"+repoBase+"--"+t.OS+"-"+t.Arch] = ver
	}
	hasShrinkwrap := false
	toolVersion := Version{
		Name:    toolName,
		Version: ver,
		Bin: map[string]string{
			"gh": "./bin/otr-launcher.mjs",
		},
		Dependencies: map[string]string{
			"@open-tool-registry/runtime": "1.0.0",
		},
		OptionalDependencies: optionalDependencies,
		Directories:          map[string]string{},
		Dist: Dist{
			Tarball: tarballURL(base, toolName, ver),
			Shasum:  "0000000000000000000000000000000000000000", // TODO: real tarball shasum once packages are materialized.
		},
		HasShrinkwrap: &hasShrinkwrap,
	}

	return reghttp.WriteJSON(w, "application/json", VersionDocument{
		Version:        toolVersion,
		Description:    "OTR package",
		License:        "MIT",
		ID:             toolName + "@" + ver,
		Readme:         "# OTR\n",
		ReadmeFilename: "README.md",
		Repository:     repo,
	})
}

// GET /{package}/-/{filename}
func (r *Registry) DownloadPackage(w http.ResponseWriter, _ *http.Request) error {
	// TODO: stream materialized tarball from storage.
	return reghttp.NotImplemented("tarball not implemented")
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

func parsePackageName(transportName string) (app.Identity, *app.Target, error) {
	local := transportName
	if i := strings.Index(local, "/"); i >= 0 {
		local = local[i+1:]
	}

	parts := strings.Split(local, "--")
	switch len(parts) {
	case 2:
		return app.Identity(parts[0] + "/" + parts[1]), nil, nil
	case 3:
		targetParts := strings.Split(parts[2], "-")
		if len(targetParts) < 2 {
			return "", nil, fmt.Errorf("invalid platform suffix: %s", transportName)
		}
		t := &app.Target{OS: targetParts[0], Arch: targetParts[1]}
		if len(targetParts) >= 3 {
			t.Libc = targetParts[2]
		}
		return app.Identity(parts[0] + "/" + parts[1]), t, nil
	default:
		return "", nil, fmt.Errorf("invalid transport name: %s", transportName)
	}
}
