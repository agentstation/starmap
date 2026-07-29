package local

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Source observes a human workspace catalog, either injected after validated
// loading or loaded from its configured path.
type Source struct {
	catalogPath     string
	snapshot        *catalogs.Catalog
	loadReport      catalogs.LoadReport
	catalogProvided bool // Track if catalog was provided via WithCatalog option
}

var _ sources.Source = (*Source)(nil)

// New creates a new local source.
func New(opts ...Option) *Source {
	s := &Source{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures a local source.
type Option func(*Source)

// WithCatalogPath sets the catalog path.
func WithCatalogPath(path string) Option {
	return func(s *Source) {
		s.catalogPath = path
	}
}

// WithCatalog sets a pre-loaded human workspace catalog to reuse.
func WithCatalog(catalog *catalogs.Catalog) Option {
	return func(s *Source) {
		s.snapshot = catalog
		s.catalogProvided = true
	}
}

// WithCatalogReport sets a pre-loaded catalog and its source load diagnostics.
func WithCatalogReport(catalog *catalogs.Catalog, report catalogs.LoadReport) Option {
	return func(s *Source) {
		s.snapshot = catalog
		s.loadReport = report
		s.catalogProvided = true
	}
}

// ID returns the ID of this source.
func (s *Source) ID() sources.ID {
	// For local source, we always return the constant name
	// The path details can be logged separately if needed
	return sources.LocalCatalogID
}

// Name returns the human-friendly name of this source.
func (s *Source) Name() string { return "Local Catalog" }

// Observe returns catalog data from the configured source without retaining result state.
func (s *Source) Observe(_ context.Context, _ ...sources.Option) (sources.Observation, error) {
	// If catalog was provided via WithCatalog option, reuse it
	if s.catalogProvided {
		return s.observation(s.snapshot, s.loadReport)
	}

	if s.catalogPath == "" {
		return sources.Observation{}, &errors.ConfigError{
			Component: "local catalog source",
			Message:   "a human workspace path or preloaded catalog is required",
		}
	}
	builder, err := catalogs.NewFromPath(s.catalogPath)
	if err != nil {
		return sources.Observation{}, errors.WrapResource("load", "human catalog", s.catalogPath, err)
	}
	builder.SetMergeStrategy(catalogs.MergeReplaceAll)
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		return sources.Observation{}, errors.WrapResource("publish", "local source observation", "", err)
	}
	return s.observation(catalog, builder.LoadReport())
}

func (s *Source) observation(catalog *catalogs.Catalog, report catalogs.LoadReport) (sources.Observation, error) {
	issues := make([]sources.ObservationIssue, 0, len(report.Issues))
	for _, issue := range report.Issues {
		code := sources.ObservationIssueCodeInvalidRecord
		scope := sources.ObservationIssueScopeRecord
		if issue.Limit {
			code = sources.ObservationIssueCodePayloadLimit
			scope = sources.ObservationIssueScopeSource
		}
		issues = append(issues, sources.ObservationIssue{
			Scope: scope, Code: code, Subject: issue.Path, Message: issue.Err.Error(),
		})
	}
	completeness := sources.ObservationCompletenessComplete
	status := sources.ObservationStatusSucceeded
	if report.Rejected > 0 {
		completeness = sources.ObservationCompletenessPartial
		status = sources.ObservationStatusDegraded
	}
	return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
		ObservedAt:   time.Now().UTC(),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: completeness,
		Status:       status,
		Records: sources.ObservationRecordCounts{
			Accepted: report.Accepted, Rejected: report.Rejected,
		},
		Issues: issues,
	})
}

// Cleanup releases any resources.
func (s *Source) Cleanup() error {
	// LocalSource doesn't hold any resources
	return nil
}

// Dependencies returns the list of external dependencies.
// Local source has no external dependencies.
func (s *Source) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional reports that a human workspace observation is optional when the
// verified embedded observation is available.
func (s *Source) IsOptional() bool {
	return true
}
