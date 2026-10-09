package npm

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/inbound/registry/reghttp"
	"github.com/ras0q/otr/outbound/storage"
)

const ContentTypeInstallV1 = "application/vnd.npm.install-v1+json"

type Config struct {
	PublicURL string
	Scope     string
}

type Registry struct {
	http.ServeMux
	service   app.Service
	storage   storage.Storage
	publicURL string
	scope     string
}

var _ http.Handler = (*Registry)(nil)

func NewRegistry(service app.Service, blobStorage storage.Storage, config Config) *Registry {
	scope := strings.TrimSpace(config.Scope)
	if scope == "" {
		scope = "@otr"
	} else if !strings.HasPrefix(scope, "@") {
		scope = "@" + scope
	}

	r := &Registry{
		service:   service,
		storage:   blobStorage,
		publicURL: strings.TrimSuffix(strings.TrimSpace(config.PublicURL), "/"),
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

	identity, target, err := parsePackageName(transportName)
	if err != nil {
		return reghttp.BadRequest(err.Error())
	}

	catalog, err := r.service.Resolve(req.Context(), identity)
	if err != nil {
		return fmt.Errorf("resolve catalog: %w", err)
	}

	baseURL := r.publicURL
	if baseURL == "" {
		if req.Host == "" {
			baseURL = "http://127.0.0.1"
		} else {
			baseURL = "http://" + req.Host
		}
	}

	modified := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	shasum, err := r.tarballShasum(req.Context(), catalog, target, string(catalog.Version))
	if err != nil {
		return fmt.Errorf("tarball shasum: %w", err)
	}
	packumentInput := PackumentInput{
		Scope:         r.scope,
		BaseURL:       baseURL,
		Catalog:       catalog,
		Target:        target,
		Modified:      modified,
		TarballShasum: shasum,
	}

	if install {
		return reghttp.WriteJSON(w, ContentTypeInstallV1, NewPackumentInstall(packumentInput))
	}
	return reghttp.WriteJSON(w, reghttp.ContentTypeJSON, NewPackumentFull(packumentInput))
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

	identity, target, err := parsePackageName(transportName)
	if err != nil {
		return reghttp.BadRequest(err.Error())
	}

	catalog, err := r.service.Resolve(req.Context(), identity)
	if err != nil {
		return fmt.Errorf("resolve catalog: %w", err)
	}

	version := req.PathValue("version")
	if version == "latest" {
		version = string(catalog.Version)
	} else if version != string(catalog.Version) {
		return reghttp.StatusError(http.StatusNotFound, "version not found")
	}

	baseURL := r.publicURL
	if baseURL == "" {
		if req.Host == "" {
			baseURL = "http://127.0.0.1"
		} else {
			baseURL = "http://" + req.Host
		}
	}

	shasum, err := r.tarballShasum(req.Context(), catalog, target, version)
	if err != nil {
		return fmt.Errorf("tarball shasum: %w", err)
	}
	packumentInput := PackumentInput{
		Scope:         r.scope,
		BaseURL:       baseURL,
		Catalog:       catalog,
		Target:        target,
		TarballShasum: shasum,
	}

	return reghttp.WriteJSON(w, reghttp.ContentTypeJSON, NewVersionDocument(packumentInput, version))
}

// GET /{package}/-/{filename}
func (r *Registry) DownloadPackage(w http.ResponseWriter, req *http.Request) error {
	transportName, err := url.PathUnescape(req.PathValue("package"))
	if err != nil {
		return reghttp.BadRequest("decode package name: " + err.Error())
	}
	if transportName == "" {
		return reghttp.BadRequest("invalid package name")
	}

	filename, err := url.PathUnescape(req.PathValue("filename"))
	if err != nil {
		return reghttp.BadRequest("decode filename: " + err.Error())
	}

	identity, target, err := parsePackageName(transportName)
	if err != nil {
		return reghttp.BadRequest(err.Error())
	}

	catalog, err := r.service.Resolve(req.Context(), identity)
	if err != nil {
		return fmt.Errorf("resolve catalog: %w", err)
	}

	packageName, _ := packumentNameAndVersion(PackumentInput{
		Scope:   r.scope,
		Catalog: catalog,
		Target:  target,
	})
	version, err := versionFromTarballFilename(packageName, filename)
	if err != nil {
		return reghttp.BadRequest(err.Error())
	}

	tarballReader, err := r.openTarball(req.Context(), catalog, target, version)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return reghttp.StatusError(http.StatusNotFound, err.Error())
		}
		return fmt.Errorf("open tarball: %w", err)
	}
	defer tarballReader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err := io.Copy(w, tarballReader); err != nil {
		return fmt.Errorf("stream tarball: %w", err)
	}
	return nil
}

func parsePackageName(transportName string) (app.Identity, *app.Target, error) {
	local := transportName
	if index := strings.Index(local, "/"); index >= 0 {
		local = local[index+1:]
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
		target := &app.Target{OS: targetParts[0], Arch: targetParts[1]}
		if len(targetParts) >= 3 {
			target.Libc = targetParts[2]
		}
		return app.Identity(parts[0] + "/" + parts[1]), target, nil
	default:
		return "", nil, fmt.Errorf("invalid transport name: %s", transportName)
	}
}
