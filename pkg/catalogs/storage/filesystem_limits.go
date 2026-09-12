package storage

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
)

// MaxFilesystemManifestBytes bounds one stored manifest before reading or publication.
// It matches the catalog distribution envelope limit.
const MaxFilesystemManifestBytes = 64 << 20

// MaxFilesystemPayloadBytes bounds a stored payload before reading or publication.
const MaxFilesystemPayloadBytes = resourcepolicy.MaxPayloadBytes

func filesystemRecordLimit(name string) (int64, error) {
	switch name {
	case currentFilename, authorityFilename:
		return catalogs.MaxCatalogAuthorityRecordBytes, nil
	case manifestFilename:
		return MaxFilesystemManifestBytes, nil
	case payloadFilename:
		return MaxFilesystemPayloadBytes, nil
	default:
		return 0, &errors.ValidationError{Field: "catalog_store.record", Value: name, Message: "is not a generation-store record"}
	}
}

func validateFilesystemRecordSize(name string, size int64) error {
	limit, err := filesystemRecordLimit(name)
	if err != nil {
		return err
	}
	if size < 0 || size > limit {
		return &errors.ValidationError{Field: "catalog_store.record_size", Value: name, Message: "record exceeds the filesystem reader limit"}
	}
	return nil
}

func prepareFilesystemManifest(ctx context.Context, generation catalogs.Generation) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateFilesystemRecordSize(currentFilename, int64(len(generation.Manifest.GenerationID))+1); err != nil {
		return nil, err
	}
	if err := validateFilesystemRecordSize(payloadFilename, int64(len(generation.Payload))); err != nil {
		return nil, err
	}
	if err := validateCandidate(ctx, generation); err != nil {
		return nil, err
	}
	manifest, err := marshalManifest(generation.Manifest)
	if err != nil {
		return nil, err
	}
	if err := validateFilesystemRecordSize(manifestFilename, int64(len(manifest))); err != nil {
		return nil, err
	}
	return manifest, nil
}
