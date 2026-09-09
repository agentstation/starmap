package pipeline

import (
	"context"
	"errors"
	"slices"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceFailureSummariesPreserveJoinedSources(t *testing.T) {
	invalid := pkgerrors.WrapResource("validate", "source observation", string(sources.ModelsDevHTTPID), &pkgerrors.ValidationError{Field: "observation", Message: "private sentinel"})
	unavailable := &pkgerrors.DependencyError{Source: string(sources.ModelsDevGitID), Message: "private dependency sentinel"}
	timeout := pkgerrors.WrapResource("observe", "source", string(sources.LocalCatalogID), context.DeadlineExceeded)
	unknown := pkgerrors.WrapResource("observe", "source", "unknown-private-source", context.DeadlineExceeded)
	failures := []error{errors.Join(invalid, unavailable, timeout, invalid, unknown)}
	actual := sourceFailureSummaries(failures)
	expected := []sources.SourceFailure{
		{Source: sources.ModelsDevHTTPID, Reason: sources.ProviderReasonResponseInvalid},
		{Source: sources.ModelsDevGitID, Reason: sources.ProviderReasonDependencyUnavailable},
		{Source: sources.LocalCatalogID, Reason: sources.ProviderReasonRequestTimeout},
	}
	if !slices.Equal(actual, expected) {
		t.Fatalf("source failures=%v", actual)
	}
}
