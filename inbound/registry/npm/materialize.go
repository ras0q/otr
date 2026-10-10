package npm

import (
	"context"
	"errors"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mholt/archives"
	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/outbound/storage"
)

//go:embed scripts/otr-launcher.mjs
var launcherScript []byte

func (r *Registry) openTarball(ctx context.Context, catalog app.Catalog, target *app.Target, version string) (io.ReadCloser, error) {
	packumentInput := PackumentInput{Scope: r.scope, Catalog: catalog, Target: target}
	packageName, _ := packumentNameAndVersion(packumentInput)
	key := tarballKey(packageName, version)

	if _, err := r.ensureTarballInfo(ctx, catalog, target, version); err != nil {
		return nil, err
	}
	reader, _, err := r.store.Open(ctx, key)
	return reader, err
}

func (r *Registry) tarballShasum(ctx context.Context, catalog app.Catalog, target *app.Target, version string) (string, error) {
	packumentInput := PackumentInput{Scope: r.scope, Catalog: catalog, Target: target}
	packageName, _ := packumentNameAndVersion(packumentInput)
	key := tarballKey(packageName, version)

	info, err := r.store.Head(ctx, key)
	if err == nil {
		return info.SHA1, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		// Packument metadata only: npm may probe many optional platform packages.
		// Materialize on GET /.../-/*.tgz, not here.
		return "", nil
	}
	return "", err
}

func (r *Registry) ensureTarballInfo(ctx context.Context, catalog app.Catalog, target *app.Target, version string) (storage.Info, error) {
	if string(catalog.Version) != version {
		return storage.Info{}, fmt.Errorf("version %q not found", version)
	}

	packumentInput := PackumentInput{Scope: r.scope, Catalog: catalog, Target: target}
	packageName, _ := packumentNameAndVersion(packumentInput)
	key := tarballKey(packageName, version)

	return r.store.PopulateIfAbsent(ctx, key, func(ctx context.Context, writer io.Writer) error {
		if target != nil {
			return r.writePlatformTarball(ctx, writer, packumentInput, packageName, version, *target)
		}
		return writeToolTarball(ctx, writer, packumentInput, packageName, version)
	})
}

func writeToolTarball(ctx context.Context, writer io.Writer, packumentInput PackumentInput, packageName, version string) error {
	repositorySlug := strings.ReplaceAll(string(packumentInput.Catalog.Identity), "/", "--")
	optionalDependencies := make(map[string]string, len(packumentInput.Catalog.Artifacts))
	for _, artifact := range packumentInput.Catalog.Artifacts {
		optionalDependencies[packumentInput.Scope+"/"+repositorySlug+"--"+artifact.OS+"-"+artifact.Arch] = version
	}

	packageManifest := map[string]any{
		"name":    packageName,
		"version": version,
		"bin": map[string]string{
			"gh": "./bin/otr-launcher.mjs",
		},
		"optionalDependencies": optionalDependencies,
	}
	packageJSON, err := json.Marshal(packageManifest)
	if err != nil {
		return fmt.Errorf("encode package.json: %w", err)
	}

	files := []archives.FileInfo{
		archiveBytes("package/package.json", 0o644, packageJSON),
		archiveBytes("package/bin/otr-launcher.mjs", 0o755, launcherScript),
	}
	return writeNPMPack(ctx, writer, files)
}

func (r *Registry) writePlatformTarball(ctx context.Context, writer io.Writer, packumentInput PackumentInput, packageName, version string, target app.Target) error {
	releaseAsset, err := r.service.OpenReleaseAsset(ctx, packumentInput.Catalog.Identity, target, version)
	if err != nil {
		return err
	}
	releaseAssetClosed := false
	defer func() {
		if !releaseAssetClosed {
			_ = releaseAsset.Body.Close()
		}
	}()

	packageManifest := map[string]any{
		"name":    packageName,
		"version": version,
		"os":      []string{target.OS},
		"cpu":     []string{target.Arch},
	}
	packageJSON, err := json.Marshal(packageManifest)
	if err != nil {
		return fmt.Errorf("encode package.json: %w", err)
	}

	binaryName, binaryData, err := binaryFromReleaseArchive(ctx, releaseAsset.Name, releaseAsset.Body)
	if err != nil {
		return err
	}
	releaseAssetClosed = true

	files := []archives.FileInfo{
		archiveBytes("package/package.json", 0o644, packageJSON),
		archiveBytes("package/asset/"+binaryName, 0o755, binaryData),
	}
	return writeNPMPack(ctx, writer, files)
}
