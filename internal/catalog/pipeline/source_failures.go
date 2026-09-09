package pipeline

import (
	"errors"
	"slices"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func sourceFailureSummaries(failures []error) []sources.SourceFailure {
	var summaries []sources.SourceFailure
	pending := slices.Clone(failures)
	for len(pending) > 0 {
		failure := pending[0]
		pending = pending[1:]
		if joined, ok := failure.(interface{ Unwrap() []error }); ok {
			pending = append(pending, joined.Unwrap()...)
			continue
		}
		var summary sources.SourceFailure
		switch failure := failure.(type) {
		case *pkgerrors.DependencyError:
			summary = sources.SourceFailure{Source: sources.ID(failure.Source), Reason: sources.ProviderReasonDependencyUnavailable}
		case *pkgerrors.ResourceError:
			if failure.Resource == "source" || failure.Resource == "source observation" {
				summary = sources.SourceFailure{Source: sources.ID(failure.ID), Reason: sources.ClassifyProviderReason(failure.Err)}
				if failure.Operation == "validate" {
					summary.Reason = sources.ProviderReasonResponseInvalid
				}
			} else if cause := errors.Unwrap(failure); cause != nil {
				pending = append(pending, cause)
			}
		default:
			if cause := errors.Unwrap(failure); cause != nil {
				pending = append(pending, cause)
			}
		}
		if summary.Valid() && !slices.Contains(summaries, summary) {
			summaries = append(summaries, summary)
		}
	}
	return summaries
}
