package runtime

import (
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// SourceDescriber declares metadata capabilities and default selection without acquisition.
// The runtime calls it once when it opens. It must not read external data.
// The runtime treats omitted metadata sources as unsupported. Eligibility remains unknown until acquisition.
type SourceDescriber interface {
	SourceConfiguration() []sources.SourceActivity
}

func describeSources(config *options) ([]sources.SourceActivity, error) {
	metadata := []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID}
	configured := make(map[sources.ID]sources.SourceActivity)
	if descriptor, ok := config.sourceAcquirer.(SourceDescriber); ok {
		for _, row := range descriptor.SourceConfiguration() {
			if _, duplicate := configured[row.Source]; duplicate || !row.Valid() || row.Source == sources.ProvidersID || row.Attempted || row.Eligibility != sources.EligibilityUnknown {
				return nil, &errors.ValidationError{Field: "source_acquirer.configuration", Message: "must declare unique passive metadata source states"}
			}
			configured[row.Source] = row
		}
	} else if config.sourceAcquirer != nil {
		for _, id := range metadata {
			configured[id] = sources.SourceActivity{Source: id, SupportUnknown: true, SelectionUnknown: true, Eligibility: sources.EligibilityUnknown}
		}
	}
	result := make([]sources.SourceActivity, 0, len(metadata)+1)
	for _, id := range append(metadata, sources.ProvidersID) {
		row, present := configured[id]
		if !present {
			row = sources.SourceActivity{Source: id, Eligibility: sources.EligibilityUnknown}
		}
		if id == sources.ProvidersID {
			row.Supported = config.acquirer != nil
			row.Enabled = row.Supported
		}
		if config.acquisitionSources != nil {
			row.Enabled = slices.Contains(config.acquisitionSources.ids, id)
			row.SelectionUnknown = false
		}
		result = append(result, row)
	}
	return result, nil
}
