package app

import (
	"context"
	"fmt"

	"github.com/aquaproj/aqua/v2/pkg/asset"
	"github.com/ras0q/otr/outbound/source"
)

type Service struct {
	source source.Source
}

func NewService(source source.Source) Service {
	return Service{source: source}
}

func (s Service) Resolve(ctx context.Context, identity Identity) (Catalog, error) {
	release, err := s.source.GetLatestRelease(ctx, identity)
	if err != nil {
		return Catalog{}, fmt.Errorf("get latest release: %w", err)
	}

	artifacts := make([]Artifact, 0, len(release.Assets))
	for _, releaseAsset := range release.Assets {
		artifact := asset.ParseAssetName(releaseAsset.Name, string(release.Version))
		artifacts = append(artifacts, Artifact(*artifact))
	}

	return Catalog{
		Identity:      identity,
		Version:       release.Version,
		RepositoryURL: release.RepositoryURL,
		Artifacts:     artifacts,
	}, nil
}

func (s Service) OpenReleaseAsset(ctx context.Context, identity Identity, target Target, version string) (ReleaseAsset, error) {
	release, err := s.source.GetLatestRelease(ctx, identity)
	if err != nil {
		return ReleaseAsset{}, fmt.Errorf("get release: %w", err)
	}
	if string(release.Version) != version {
		return ReleaseAsset{}, fmt.Errorf("version %q not found", version)
	}

	for _, releaseAsset := range release.Assets {
		info := asset.ParseAssetName(releaseAsset.Name, version)
		if info == nil {
			continue
		}
		if info.OS != target.OS || info.Arch != target.Arch {
			continue
		}
		body, err := s.source.DownloadReleaseAsset(ctx, releaseAsset.URL)
		if err != nil {
			return ReleaseAsset{}, fmt.Errorf("download asset: %w", err)
		}
		return ReleaseAsset{Name: releaseAsset.Name, Size: releaseAsset.Size, Body: body}, nil
	}
	return ReleaseAsset{}, fmt.Errorf("no release asset for %s/%s", target.OS, target.Arch)
}
