package npm

import (
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
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
	if string(catalog.Version) != version {
		return nil, fmt.Errorf("version %q not found", version)
	}

	packumentInput := PackumentInput{Scope: r.scope, Catalog: catalog, Target: target}
	packageName, _ := packumentNameAndVersion(packumentInput)
	storageKey := tarballStorageKey(packageName, version)

	shasumKey := tarballShasumStorageKey(packageName, version)

	return storage.OpenCached(ctx, r.storage, storageKey, func(ctx context.Context, writer io.Writer) error {
		hasher := sha1.New()
		tracked := io.MultiWriter(writer, hasher)

		var err error
		if target != nil {
			err = r.writePlatformTarball(ctx, tracked, packumentInput, packageName, version, *target)
		} else {
			err = writeToolTarball(ctx, tracked, packumentInput, packageName, version)
		}
		if err != nil {
			return err
		}

		shasum := hex.EncodeToString(hasher.Sum(nil))
		return r.storage.Put(ctx, shasumKey, strings.NewReader(shasum))
	})
}

func (r *Registry) tarballShasum(ctx context.Context, catalog app.Catalog, target *app.Target, version string) (string, error) {
	packumentInput := PackumentInput{Scope: r.scope, Catalog: catalog, Target: target}
	packageName, _ := packumentNameAndVersion(packumentInput)
	shasumKey := tarballShasumStorageKey(packageName, version)
	tarballKey := tarballStorageKey(packageName, version)

	if shasum, err := readStorageText(ctx, r.storage, shasumKey); err == nil && shasum != "" {
		return shasum, nil
	}

	if ok, err := r.storage.Exists(ctx, tarballKey); err != nil {
		return "", err
	} else if ok {
		return r.writeTarballShasumFromStored(ctx, packageName, version)
	}

	if _, err := r.openTarball(ctx, catalog, target, version); err != nil {
		return "", err
	}
	return readStorageText(ctx, r.storage, shasumKey)
}

func (r *Registry) writeTarballShasumFromStored(ctx context.Context, packageName, version string) (string, error) {
	tarballKey := tarballStorageKey(packageName, version)
	shasumKey := tarballShasumStorageKey(packageName, version)

	reader, err := r.storage.Get(ctx, tarballKey)
	if err != nil {
		return "", err
	}
	defer func() {
		if closer, ok := reader.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("hash tarball: %w", err)
	}
	shasum := hex.EncodeToString(hasher.Sum(nil))
	if err := r.storage.Put(ctx, shasumKey, strings.NewReader(shasum)); err != nil {
		return "", err
	}
	return shasum, nil
}

func readStorageText(ctx context.Context, backend storage.Storage, key string) (string, error) {
	reader, err := backend.Get(ctx, key)
	if err != nil {
		return "", err
	}
	defer func() {
		if closer, ok := reader.(io.Closer); ok {
			_ = closer.Close()
		}
	}()
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
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
