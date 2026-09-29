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
