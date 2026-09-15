package publication

import (
	"errors"
	"time"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Dependency failures can reach admission so policy can select retained evidence.
// Every leaf must identify a dependency. Other preflight failures remain fatal.
func collectDependencyFailure(err error, startedAt time.Time) *pipeline.Collected {
	pending := []error{err}
	dependencies := 0
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == nil {
			continue
		}
		if _, ok := current.(*pkgerrors.DependencyError); ok {
			dependencies++
			continue
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			children := joined.Unwrap()
			if len(children) == 0 {
				return nil
			}
			pending = append(pending, children...)
			continue
		}
		cause := errors.Unwrap(current)
		if cause == nil {
			return nil
		}
		pending = append(pending, cause)
	}
	if dependencies == 0 {
		return nil
	}
	return &pipeline.Collected{
		StartedAt: startedAt, CompletedAt: time.Now().UTC(),
		SourceFailures: []error{err}, SourceActivities: sources.ActivityFromError(err),
		ProviderAttempts: sources.ProviderAttemptsFromError(err),
	}
}
