package runtime

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// RetainedFile identifies bytes in a verified backup. Names are slash-separated relative paths.
type RetainedFile = privatefiles.RetainedFile

// InspectRetainedPublications selects native journals and matching staging files to keep inactive.
// The caller must supply the complete verified runtime inventory and preserve all selected bytes.
// Destination records remain active and require separate owner validation. This check never promotes staging files.
// Native file identities are historical. This result does not authorize cleanup or admission.
func InspectRetainedPublications(ctx context.Context, files map[string]RetainedFile, read RetainedRecordReader) ([]string, error) {
	if len(files) > migrationSourceMaxEntries {
		return nil, invalidInputPublication("retained publication requires bounded verified input")
	}
	return privatefiles.InspectRetainedPublications(ctx, files, read, runtimePublicationDestination)
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
