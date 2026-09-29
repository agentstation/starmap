package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// RetainedFile identifies bytes in a verified backup. Names are slash-separated relative paths.
type RetainedFile struct {
	Size   int64
	SHA256 string
}

// InspectRetainedPublications selects native journals and matching staging files to keep inactive.
// The caller must supply the complete verified runtime inventory and preserve all selected bytes.
// Destination records remain active and require separate owner validation. This check never promotes staging files.
// Native file identities are historical. This result does not authorize cleanup or admission.
func InspectRetainedPublications(ctx context.Context, files map[string]RetainedFile, read RetainedRecordReader) ([]string, error) {
	if ctx == nil || read == nil || len(files) > migrationSourceMaxEntries {
		return nil, invalidInputPublication("retained publication requires bounded verified input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var journals []string
	namesBytes := 0
	for name, file := range files {
		namesBytes += len(name)
		if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") || namesBytes > migrationManifestMaxBytes || file.Size < 0 || !validRetainedFileDigest(file.SHA256) {
			return nil, invalidInputPublication("retained publication inventory is invalid")
		}
		if isRecordPublicationDirectory(path.Dir(name)) && path.Base(name) != directoryLockName {
			journals = append(journals, name)
		}
	}
	slices.Sort(journals)
	inactive := make(map[string]bool)
	for _, name := range journals {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := read(ctx, name, privatefiles.RetainedPublicationMaxBytes)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		if file := files[name]; file.Size != int64(len(raw)) || file.SHA256 != hex.EncodeToString(digest[:]) {
			return nil, invalidInputPublication("retained publication changed after backup verification")
		}
		journal, err := privatefiles.InspectRetainedPublication(path.Base(name), raw)
		if err != nil {
			return nil, err
		}
		parent := path.Dir(path.Dir(name))
		if !runtimePublicationDestination(parent, journal.Destination, journal.Prefix) {
			return nil, invalidInputPublication("retained publication destination is not owned by runtime")
		}
		lock, exists := files[path.Join(path.Dir(name), directoryLockName)]
		if !exists || lock.Size != 0 || lock.SHA256 != hex.EncodeToString(sha256.New().Sum(nil)) {
			return nil, invalidInputPublication("retained publication writer record is missing or invalid")
		}
		stage := path.Join(parent, journal.Stage)
		if file, exists := files[stage]; exists {
			if inactive[stage] || !journal.HasRecord || file.Size != journal.Size || file.SHA256 != journal.SHA256 {
				return nil, invalidInputPublication("retained staging bytes differ from the publication record")
			}
			inactive[stage] = true
		}
		inactive[name] = true
	}
	result := make([]string, 0, len(inactive))
	for name := range inactive {
		result = append(result, name)
	}
	slices.Sort(result)
	return result, ctx.Err()
}

func runtimePublicationDestination(parent, destination, prefix string) bool {
	switch parent {
	case layerDirectoryName:
		if prefix != ".layer-" {
			return false
		}
		switch destination {
		case sourceLayerFileName, manualHistoryName, removalPolicyName, inputPublicationName, permissionCheckpointFile, generationPinRecordFile:
			return true
		}
	case layerDirectoryName + "/" + providerLayerDirectoryName:
		id, ok := strings.CutSuffix(destination, ".json")
		return prefix == ".layer-" && ok && validateProviderLayerID(catalogs.ProviderID(id)) == nil
	case layerDirectoryName + "/" + providerLayerDirectoryName + "/" + bindingLayerDirectoryName:
		return prefix == ".layer-" && validInputReference(destination)
	case layerDirectoryName + "/" + inputPublicationDirectory:
		return prefix == ".input-" && validInputReference(destination)
	case "github-catalog-source":
		return prefix == ".state-" && validInputReference(destination)
	}
	return false
}

func validRetainedFileDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}
