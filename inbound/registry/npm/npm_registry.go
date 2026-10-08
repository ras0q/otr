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

const ContentTypeInstallV1 = "application/vnd.npm.install-v1+json"

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
	install := reghttp.Accepts(req.Header.Get("Accept"), ContentTypeInstallV1)

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

	modified := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	packIn := PackumentInput{
		Scope:    r.scope,
		BaseURL:  base,
		Catalog:  catalog,
		Target:   target,
		Modified: modified,
	}

	if install {
		return reghttp.WriteJSON(w, ContentTypeInstallV1, NewPackumentInstall(packIn))
	}
	return reghttp.WriteJSON(w, reghttp.ContentTypeJSON, NewPackumentFull(packIn))
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

	packIn := PackumentInput{
		Scope:   r.scope,
		BaseURL: base,
		Catalog: catalog,
		Target:  target,
	}

	return reghttp.WriteJSON(w, reghttp.ContentTypeJSON, NewVersionDocument(packIn, ver))
}

// GET /{package}/-/{filename}
func (r *Registry) DownloadPackage(w http.ResponseWriter, _ *http.Request) error {
	// TODO: stream materialized tarball from storage.
	return reghttp.NotImplemented("tarball not implemented")
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
