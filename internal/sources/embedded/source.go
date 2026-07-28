// Package embedded exposes the verified compiled catalog as a versioned,
// lowest-authority synchronization observation.
package embedded

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Source observes one verified catalog compiled into the running binary.
type Source struct {
	catalog *catalogs.Catalog
	now     func() time.Time
}

var _ sources.Source = (*Source)(nil)

// New returns an embedded source over catalog.
func New(catalog *catalogs.Catalog) *Source {
	return &Source{catalog: catalog, now: time.Now}
}

// ID returns the stable embedded catalog source identity.
func (s *Source) ID() sources.ID {
	return sources.EmbeddedCatalogID
}

// Name returns the human-readable source name.
func (s *Source) Name() string {
	return "Verified Embedded Catalog"
}

// Observe returns the immutable embedded catalog with content-addressed
// revision evidence.
func (s *Source) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return sources.Observation{}, err
	}
	if s == nil || s.catalog == nil {
		return sources.Observation{}, &errors.ValidationError{
			Field:   "embedded_source.catalog",
			Message: "verified catalog is required",
		}
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	observedAt := now().UTC()
	return sources.NewObservation(s.ID(), s.catalog, sources.ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{
			Accepted: providerModelCount(s.catalog),
		},
	})
}

func providerModelCount(catalog *catalogs.Catalog) int {
	count := 0
	for _, provider := range catalog.Providers().List() {
		count += len(provider.Models)
	}
	return count
}

// Cleanup releases source resources. The embedded source owns none.
func (s *Source) Cleanup() error {
	return nil
}

// Dependencies reports no external runtime dependencies.
func (s *Source) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional reports that a selected embedded observation has no optional
// dependency that permits skipping it.
func (s *Source) IsOptional() bool {
	return false
}
