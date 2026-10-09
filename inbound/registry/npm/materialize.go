package npm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/mholt/archives"
	"github.com/ras0q/otr/app"
	"github.com/ras0q/otr/outbound/storage"
)

//go:embed scripts/otr-launcher.mjs
var launcherScript []byte

func tarballStorageKey(packageName, version string) string {
	return path.Join("tarballs", packageName, version+".tgz")
}

func (r *Registry) openTarball(ctx context.Context, catalog app.Catalog, target *app.Target, version string) (io.ReadCloser, error) {
	if string(catalog.Version) != version {
		return nil, fmt.Errorf("version %q not found", version)
	}

	packumentInput := PackumentInput{Scope: r.scope, Catalog: catalog, Target: target}
	packageName, _ := packumentNameAndVersion(packumentInput)
	storageKey := tarballStorageKey(packageName, version)

	return storage.OpenCached(ctx, r.storage, storageKey, func(ctx context.Context, writer io.Writer) error {
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

	assetPath := "package/asset/" + releaseAsset.Name
	files := []archives.FileInfo{
		archiveBytes("package/package.json", 0o644, packageJSON),
		archiveStream(assetPath, 0o644, releaseAsset.Size, func() (io.ReadCloser, error) {
			return releaseAsset.Body, nil
		}),
	}
	if err := writeNPMPack(ctx, writer, files); err != nil {
		return err
	}
	releaseAssetClosed = true
	return nil
}
