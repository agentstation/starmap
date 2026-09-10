package main

import (
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

// validatePublishedSuccessor binds rename history to the release that the
// current channel selects. The workflow verifies both publisher attestations.
func validatePublishedSuccessor(current artifact.Channel, directory string, next catalogs.Generation) error {
	directory = strings.TrimSpace(directory)
	if current.Sequence == 0 {
		if directory != "" {
			return channelFlagError("previous_release_dir", "requires a current channel document")
		}
		return nil
	}
	if directory == "" {
		return channelFlagError("previous_release_dir", "is required for an existing channel")
	}
	previous, err := readReleaseGeneration(directory)
	if err != nil {
		return err
	}
	digest, err := previous.SemanticChecksum()
	if err != nil {
		return err
	}
	if previous.Manifest.GenerationID != current.GenerationID || digest != current.CatalogDigest {
		return channelFlagError("previous_release_dir", "does not match the current channel identity")
	}
	release, err := readReleaseDirectory(directory)
	if err != nil {
		return err
	}
	assets, err := channelAssets(directory, release.files)
	if err != nil {
		return err
	}
	if len(assets) != len(current.Assets) {
		return channelFlagError("previous_release_dir", "does not match the current channel assets")
	}
	expected := make(map[string]artifact.ChannelAsset, len(current.Assets))
	for _, asset := range current.Assets {
		expected[asset.Name] = asset
	}
	for _, asset := range assets {
		if expected[asset.Name] != asset {
			return channelFlagError("previous_release_dir", "does not match the current channel asset bytes")
		}
	}
	oldCatalog, err := catalogs.DecodeCatalogGeneration(previous)
	if err != nil {
		return err
	}
	newCatalog, err := catalogs.DecodeCatalogGeneration(next)
	if err != nil {
		return err
	}
	return oldCatalog.CanonicalAliases().ValidateSuccessor(newCatalog.CanonicalAliases())
}

func readReleaseGeneration(directory string) (catalogs.Generation, error) {
	release, err := readReleaseDirectory(directory)
	if err != nil {
		return catalogs.Generation{}, err
	}
	return artifact.Open(release.archive, release.statement)
}
