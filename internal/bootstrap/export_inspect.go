package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// InspectExports validates a private tree of completed baseline exports without writes.
// The caller must fence writers and verify the complete inventory before and after inspection.
// This check refuses journals and unfinished stages, which require separate recovery.
// A valid export does not establish provenance, authority, or permission to activate it.
func InspectExports(ctx context.Context, path string) (resultErr error) {
	if ctx == nil {
		return baselineInspectionError("inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return err
	}
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	entries, err := baselineRecoveryEntries(root, baselineRecoveryEntryLimit)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return baselineInspectionError("no completed baseline exports are present")
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		decoded, err := hex.DecodeString(entry.Name())
		if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != entry.Name() || !entry.IsDir() {
			return baselineInspectionError("entry is not a completed baseline export")
		}
		child, err := directory.ExistingChild(entry.Name())
		if err != nil {
			return err
		}
		if err := inspectExport(ctx, child, entry.Name()); err != nil {
			return err
		}
	}
	verified, err := directory.Open()
	if err != nil {
		return err
	}
	return stderrors.Join(verified.Close(), ctx.Err())
}

func inspectExport(ctx context.Context, directory *privatefiles.Directory, name string) (resultErr error) {
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	entries, err := baselineRecoveryEntries(root, 2)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return baselineInspectionError("each export requires exactly a manifest and payload")
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || entry.Name() != baselineManifestName && entry.Name() != baselinePayloadName {
			return baselineInspectionError("export contains an unsupported entry")
		}
	}
	manifestBytes, err := directory.ReadFile(baselineManifestName, catalogs.MaxCatalogPayloadBytes)
	if err != nil {
		return err
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestBytes)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(manifest.GenerationID))
	if name != hex.EncodeToString(digest[:]) {
		return baselineInspectionError("directory name differs from the generation identity")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := directory.ReadFile(baselinePayloadName, catalogs.MaxCatalogPayloadBytes)
	if err != nil {
		return err
	}
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
		return err
	}
	verified, err := directory.Open()
	if err != nil {
		return err
	}
	return stderrors.Join(verified.Close(), ctx.Err())
}

func baselineInspectionError(message string) error {
	return &errors.ValidationError{Field: "baseline.exports", Message: message}
}
