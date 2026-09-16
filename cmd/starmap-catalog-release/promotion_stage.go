package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/bootstrap/manifest"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
)

// stagePromotionDirectory projects verified release bytes into a new directory.
// The command accepts an existing directory only when it verifies an exact retry.
func stagePromotionDirectory(path, releasePath string) (promotionReport, error) {
	return (promotionStager{syncDirectory: filepublish.SyncDirectory}).stage(path, releasePath)
}

type promotionStager struct {
	syncDirectory func(*os.Root) error
}

func (p promotionStager) stage(path, releasePath string) (promotionReport, error) {
	if path == "" || releasePath == "" {
		return promotionReport{}, &errors.ValidationError{
			Field: "catalog_release.promotion", Message: "catalog and release directories are required",
		}
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return promotionReport{}, err
	}
	if _, err := os.Lstat(target); err == nil {
		return p.confirmExisting(target, releasePath)
	} else if !os.IsNotExist(err) {
		return promotionReport{}, errors.WrapIO("inspect", target, err)
	}
	release, err := readReleaseDirectory(releasePath)
	if err != nil {
		return promotionReport{}, err
	}
	generation, err := artifact.Open(release.archive, release.statement)
	if err != nil {
		return promotionReport{}, err
	}
	catalog, err := catalogs.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return promotionReport{}, err
	}
	bootstrap, _, err := manifest.DeriveCommitted(catalog, generation, nil)
	if err != nil {
		return promotionReport{}, err
	}
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return promotionReport{}, errors.WrapIO("open", filepath.Dir(target), err)
	}
	defer func() { _ = parent.Close() }()
	stagePath, err := os.MkdirTemp(filepath.Dir(target), ".starmap-promotion-")
	if err != nil {
		return promotionReport{}, err
	}
	defer func() { _ = os.RemoveAll(stagePath) }()
	candidate := filepath.Join(stagePath, "catalog")
	if _, err := workspace.Project(context.Background(), candidate, catalog, workspace.Identity{
		GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
	}); err != nil {
		return promotionReport{}, err
	}
	if err := writeStagedPromotionManifest(candidate, bootstrap); err != nil {
		return promotionReport{}, err
	}
	if err := writeStagedPromotionMetadata(candidate, catalogs.BootstrapGenerationManifestFilename, generation.Manifest); err != nil {
		return promotionReport{}, err
	}
	report, err := verifyPromotionDirectory(candidate, releasePath)
	if err != nil {
		return promotionReport{}, err
	}
	if report.ArchiveChecksum != release.archiveChecksum {
		return promotionReport{}, &errors.ConflictError{Resource: "promotion release", Message: "release changed during staging"}
	}
	stage, err := os.OpenRoot(stagePath)
	if err != nil {
		return promotionReport{}, err
	}
	defer func() { _ = stage.Close() }()
	if err := filepublish.DirectoryBetweenRootsNoReplace(stage, "catalog", parent, filepath.Base(target)); err != nil {
		return promotionReport{}, err
	}
	for _, directory := range []*os.Root{stage, parent} {
		if err := p.syncDirectory(directory); err != nil {
			return promotionReport{}, &errors.PublicationError{Resource: "promotion catalog", ID: target, Err: err}
		}
	}
	report.CatalogDirectory = target
	return report, nil
}

func writeStagedPromotionManifest(path string, bootstrap catalogs.BootstrapManifest) error {
	return writeStagedPromotionMetadata(path, "generation.json", bootstrap)
}

func writeStagedPromotionMetadata(path, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, constants.FilePermissions)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return filepublish.SyncDirectory(root)
}

func (p promotionStager) confirmExisting(target, releasePath string) (promotionReport, error) {
	report, err := verifyPromotionDirectory(target, releasePath)
	if err != nil {
		return promotionReport{}, err
	}
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return promotionReport{}, err
	}
	defer func() { _ = parent.Close() }()
	if err := p.syncDirectory(parent); err != nil {
		return promotionReport{}, &errors.PublicationError{Resource: "promotion catalog", ID: target, Err: err}
	}
	return report, nil
}
