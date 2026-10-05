package app

import (
	"context"
	"fmt"

	"github.com/aquaproj/aqua/v2/pkg/asset"
	"github.com/ras0q/otr/outbound/source"
	"github.com/ras0q/otr/outbound/storage"
)

type Resolver struct {
	source  source.Source
	storage storage.Storage
}

func NewResolver(source source.Source, storage storage.Storage) Resolver {
	return Resolver{source: source, storage: storage}
}

func (r Resolver) Resolve(ctx context.Context, id Identity) (Catalog, error) {
	// TODO: cache catalog metadata and materialized packages in r.storage.
	release, err := r.source.GetLatestRelease(ctx, id)
	if err != nil {
		return Catalog{}, fmt.Errorf("get latest release: %w", err)
	}

	artifacts := make([]Artifact, 0, len(release.Assets))
	for _, a := range release.Assets {
		artifact := asset.ParseAssetName(a.Name, string(release.Version))
		artifacts = append(artifacts, Artifact(*artifact))
	}

	return Catalog{
		Identity:      id,
		Version:       release.Version,
		RepositoryURL: release.RepositoryURL,
		Artifacts:     artifacts,
	}, nil
}
