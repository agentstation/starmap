package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap/manifest"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func deriveMetadata(catalog *catalogs.Catalog, directory, storePath string, current *catalogs.BootstrapManifest, now time.Time) (catalogs.BootstrapManifest, manifest.Report, *catalogs.Generation, error) {
	committed, missingStore, err := committedInput(catalog, directory, storePath)
	if err != nil {
		return catalogs.BootstrapManifest{}, manifest.Report{}, nil, err
	}
	var bootstrap catalogs.BootstrapManifest
	var report manifest.Report
	if committed != nil {
		bootstrap, report, err = manifest.DeriveCommitted(catalog, *committed, current)
	} else if len(catalog.MembershipScopes()) != 0 {
		err = &errors.ValidationError{Field: "bootstrap_manifest.committed_generation", Message: "membership evidence requires its committed generation; supply generation-store"}
	} else {
		bootstrap, report, err = manifest.Derive(catalog, current, now)
	}
	if err == nil && missingStore && report.Changed {
		err = &errors.ValidationError{Field: "bootstrap_manifest.committed_generation", Value: storePath, Message: "changed catalog has no committed generation"}
	}
	return bootstrap, report, committed, err
}

func committedInput(catalog *catalogs.Catalog, directory, storePath string) (*catalogs.Generation, bool, error) {
	missingStore := false
	if storePath != "" {
		store, err := storage.NewFilesystem(storePath)
		if err != nil {
			return nil, false, err
		}
		generation, err := store.Current(context.Background())
		if err == nil {
			return &generation, false, nil
		}
		if !errors.IsNotFound(err) {
			return nil, false, errors.WrapResource("read", "committed catalog generation", storePath, err)
		}
		missingStore = true
	}
	name := filepath.Join(directory, catalogs.BootstrapGenerationManifestFilename)
	data, err := os.ReadFile(name) //nolint:gosec // Explicit repository tooling path.
	if os.IsNotExist(err) {
		return nil, missingStore, nil
	}
	if err != nil {
		return nil, missingStore, errors.WrapIO("read", name, err)
	}
	retained, err := catalogs.ParseGenerationManifestJSON(data)
	if err != nil {
		return nil, missingStore, err
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return nil, missingStore, err
	}
	return &catalogs.Generation{Manifest: retained, Payload: payload}, missingStore, nil
}

func writeCommittedMetadata(manifestPath string, generation *catalogs.Generation) error {
	name := filepath.Join(filepath.Dir(manifestPath), catalogs.BootstrapGenerationManifestFilename)
	if generation == nil {
		if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
			return errors.WrapIO("remove", name, err)
		}
		return nil
	}
	data, err := json.MarshalIndent(generation.Manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	previous, err := os.ReadFile(name) //nolint:gosec // Explicit repository tooling path.
	if err != nil && !os.IsNotExist(err) {
		return errors.WrapIO("read", name, err)
	}
	if bytes.Equal(previous, data) {
		return nil
	}
	return writeAtomic(name, data)
}
