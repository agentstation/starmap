package catalogartifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// ChecksumFilename is the detached SHA-256 checksum asset.
	ChecksumFilename = "starmap-catalog.tar.gz.sha256"
	releaseDirectory = "catalog-generations"
)

// ReleaseAssets describes one atomically staged immutable publication set.
type ReleaseAssets struct {
	GenerationID    string
	ArchiveChecksum string
	Directory       string
	Files           []string
}

// StageReleaseAssets validates and atomically stages archive, attestation, and
// checksum assets. An exact retry is idempotent; rebinding the same generation
// ID to different bytes returns a typed conflict.
func StageReleaseAssets(root string, artifact Artifact) (ReleaseAssets, error) {
	if strings.TrimSpace(root) == "" {
		return ReleaseAssets{}, &errors.ValidationError{Field: "catalog_artifact.release_root", Message: "is required"}
	}
	generation, err := Open(artifact.Data, artifact.Attestation)
	if err != nil {
		return ReleaseAssets{}, err
	}
	if artifact.GenerationID != generation.Manifest.GenerationID || artifact.Filename != Filename ||
		artifact.AttestationFilename != AttestationFilename || artifact.MediaType != MediaType ||
		artifact.Checksum != checksum(artifact.Data) {
		return ReleaseAssets{}, artifactValidation("release", artifact.GenerationID, "metadata does not match verified artifact bytes")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return ReleaseAssets{}, errors.WrapIO("resolve", root, err)
	}
	base := filepath.Join(absoluteRoot, releaseDirectory)
	if err := os.MkdirAll(base, constants.DirPermissions); err != nil {
		return ReleaseAssets{}, errors.WrapIO("create", base, err)
	}
	target := filepath.Join(base, releaseDirectoryName(artifact.GenerationID))
	assets := []releaseAsset{
		{name: Filename, data: artifact.Data},
		{name: AttestationFilename, data: artifact.Attestation},
		{name: ChecksumFilename, data: []byte(strings.TrimPrefix(artifact.Checksum, "sha256:") + "  " + Filename + "\n")},
	}
	if err := verifyExistingRelease(target, assets); err == nil {
		return releaseAssets(artifact.GenerationID, target, assets), nil
	} else if !os.IsNotExist(err) {
		return ReleaseAssets{}, err
	}

	temporary, err := os.MkdirTemp(base, ".candidate-")
	if err != nil {
		return ReleaseAssets{}, errors.WrapIO("create", base, err)
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	if err := os.Chmod(temporary, constants.DirPermissions); err != nil {
		return ReleaseAssets{}, errors.WrapIO("chmod", temporary, err)
	}
	for _, asset := range assets {
		if err := writeReleaseFile(filepath.Join(temporary, asset.name), asset.data); err != nil {
			return ReleaseAssets{}, err
		}
	}
	if err := syncReleaseDirectory(temporary); err != nil {
		return ReleaseAssets{}, errors.WrapIO("sync", temporary, err)
	}
	if err := os.Rename(temporary, target); err != nil {
		if existingErr := verifyExistingRelease(target, assets); existingErr == nil {
			return releaseAssets(artifact.GenerationID, target, assets), nil
		}
		return ReleaseAssets{}, errors.WrapIO("publish", target, err)
	}
	if err := syncReleaseDirectory(base); err != nil {
		return ReleaseAssets{}, errors.WrapIO("sync", base, err)
	}
	return releaseAssets(artifact.GenerationID, target, assets), nil
}

type releaseAsset struct {
	name string
	data []byte
}

func verifyExistingRelease(target string, assets []releaseAsset) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &errors.ConflictError{Resource: "catalog release generation", Expected: target, Actual: "non-directory", Message: "immutable release path is occupied"}
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return errors.WrapIO("read", target, err)
	}
	if len(entries) != len(assets) {
		return releaseConflict(target)
	}
	for _, asset := range assets {
		data, err := os.ReadFile(filepath.Join(target, asset.name)) //nolint:gosec // target is digest-derived beneath caller root.
		if err != nil || !bytes.Equal(data, asset.data) {
			return releaseConflict(target)
		}
	}
	return nil
}

func releaseConflict(target string) error {
	return &errors.ConflictError{
		Resource: "catalog release generation", Expected: target, Actual: target,
		Message: "generation ID is already staged with different immutable assets",
	}
}

func releaseDirectoryName(generationID string) string {
	digest := sha256.Sum256([]byte(generationID))
	return hex.EncodeToString(digest[:])
}

func writeReleaseFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, constants.FilePermissions) //nolint:gosec // path is fixed beneath a private staging directory.
	if err != nil {
		return errors.WrapIO("create", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return errors.WrapIO("write", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return errors.WrapIO("sync", path, err)
	}
	if err := file.Close(); err != nil {
		return errors.WrapIO("close", path, err)
	}
	return nil
}

func syncReleaseDirectory(path string) error {
	directory, err := os.Open(path) //nolint:gosec // path is owned by release staging.
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}

func releaseAssets(generationID, directory string, assets []releaseAsset) ReleaseAssets {
	files := make([]string, 0, len(assets))
	for _, asset := range assets {
		files = append(files, filepath.Join(directory, asset.name))
	}
	return ReleaseAssets{
		GenerationID: generationID, ArchiveChecksum: checksum(assets[0].data),
		Directory: directory, Files: files,
	}
}
