package sources

import "github.com/agentstation/starmap/pkg/errors"

// IsExactGitCommit reports whether value is a complete SHA-1 or SHA-256 Git commit.
func IsExactGitCommit(value string) bool {
	return (len(value) == 40 || len(value) == 64) && isHex(value)
}

// ValidateAcquisitionSelection checks the explicit local acquisition source set.
// An empty set permits no acquisition. Distribution baselines remain separate.
func ValidateAcquisitionSelection(ids []ID) error {
	seen := make(map[ID]bool, len(ids))
	for _, id := range ids {
		switch id {
		case ProvidersID, LocalCatalogID, ModelsDevHTTPID, ModelsDevGitID:
		default:
			return &errors.ValidationError{Field: "acquisition_sources", Message: "must select providers, local_catalog, models_dev_http, or models_dev_git"}
		}
		if seen[id] {
			return &errors.ValidationError{Field: "acquisition_sources", Message: "source IDs must not repeat"}
		}
		seen[id] = true
	}
	if seen[ModelsDevHTTPID] && seen[ModelsDevGitID] {
		return &errors.ValidationError{Field: "acquisition_sources", Message: "must select only one models.dev source form"}
	}
	return nil
}
