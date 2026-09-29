package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/bootstrap"
)

// BaselineExport identifies the immutable embedded export and any recovery work.
type BaselineExport = bootstrap.ExportResult

// BaselineRecovery reports recovered operations and preserved paths within the export directory.
type BaselineRecovery = bootstrap.BaselineRecovery

// ExportEmbeddedBaseline writes the binary's catalog to an explicit host directory.
// It preserves conflicting files and never changes an accepted catalog or starts acquisition.
// The directory must be absolute. Repeated calls verify the existing export.
func ExportEmbeddedBaseline(ctx context.Context, directory string) (BaselineExport, error) {
	return bootstrap.Export(ctx, directory)
}

// InspectBaselineExports validates a private tree of completed baseline exports without writes.
// Each child must have its generation-derived name, manifest, and compatible catalog payload.
// The caller must fence writers and verify complete inventory before and after inspection.
// Exclude journals and unfinished stages. This check does not approve provenance or activation.
func InspectBaselineExports(ctx context.Context, directory string) error {
	return bootstrap.InspectExports(ctx, directory)
}

// BaselineRetainedFile identifies captured bytes in a verified baseline inventory.
type BaselineRetainedFile = bootstrap.BaselineRetainedFile

// BaselineRecordReader reads bounded metadata from a verified backup.
type BaselineRecordReader = bootstrap.BaselineRecordReader

// InspectRetainedBaselinePublications selects matching baseline stages for inactive retention.
// Include the baseline files and the separate .starmap-baseline recovery records.
// Preserve selected files and all recovery records in inactive evidence. Validate completed exports separately.
// This check never promotes staging bytes or authorizes native cleanup or admission.
func InspectRetainedBaselinePublications(ctx context.Context, files map[string]BaselineRetainedFile, read BaselineRecordReader) ([]string, error) {
	return bootstrap.InspectRetainedBaselinePublications(ctx, files, read)
}

// BaselineRecoveryDirectoryName identifies the journal directory within a baseline export tree.
const BaselineRecoveryDirectoryName = bootstrap.BaselineRecoveryDirectoryName
