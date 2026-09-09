package pipeline

import (
	"errors"
	"slices"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func sourceFailureSummaries(failures []error) []sources.SourceFailure {
	var summaries []sources.SourceFailure
	for _, failure := range failures {
		var dependency *pkgerrors.DependencyError
		if !errors.As(failure, &dependency) {
			continue
		}
		summary := sources.SourceFailure{Source: sources.ID(dependency.Source), Reason: sources.ProviderReasonDependencyUnavailable}
		if summary.Valid() && !slices.Contains(summaries, summary) {
			summaries = append(summaries, summary)
		}
	}
	return summaries
}
