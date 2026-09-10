package local

import (
	"context"
	"slices"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Source observes a human catalog workspace catalog, either injected after validated
// loading or loaded from its configured path.
type Source struct {
	catalogPath     string
	snapshot        *catalogs.Catalog
	loadReport      catalogs.LoadReport
	catalogProvided bool
	aliasBaseline   *catalogs.Catalog
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

// WithCatalog sets a pre-loaded human catalog workspace catalog to reuse.
func WithCatalog(catalog *catalogs.Catalog) Option {
	return func(s *Source) {
		s.snapshot = catalog
		s.catalogProvided = true
	}
}

// WithAliasBaseline permits unchanged projected rename records from the accepted baseline.
// Local acquisition cannot change that inventory or grant canonical rename authority.
func WithAliasBaseline(baseline *catalogs.Catalog) Option {
	return func(s *Source) { s.aliasBaseline = baseline }
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
	return sources.LocalCatalogID
}

// Name returns the human-friendly name of this source.
func (s *Source) Name() string { return "Local Catalog" }

// Observe returns catalog data from the configured source without retaining result state.
func (s *Source) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	if s.catalogProvided {
		return s.observation(s.snapshot, s.loadReport)
	}

	if s.catalogPath == "" {
		return sources.Observation{}, &errors.ConfigError{
			Component: "local catalog source",
			Message:   "a human workspace path or preloaded catalog is required",
		}
	}
	var observation sources.Observation
	err := workspace.Read(ctx, s.catalogPath, func(workspace.InputExpectation) error {
		builder, err := catalogs.NewFromPath(s.catalogPath)
		if err != nil {
			return errors.WrapResource("load", "human catalog", s.catalogPath, err)
		}
		builder.SetMergeStrategy(catalogs.MergeReplaceAll)
		if err := s.validateAndClearAliases(builder); err != nil {
			return err
		}
		catalog, err := catalogs.NewObservationCatalog(builder)
		if err != nil {
			return errors.WrapResource("publish", "local source observation", "", err)
		}
		observation, err = s.observation(catalog, builder.LoadReport())
		return err
	})
	if err != nil {
		return sources.Observation{}, err
	}
	return observation, nil
}

func (s *Source) observation(catalog *catalogs.Catalog, report catalogs.LoadReport) (sources.Observation, error) {
	if catalog != nil && len(catalog.CanonicalAliasRecords()) != 0 {
		builder, err := catalogs.NewBuilderFrom(catalog)
		if err != nil {
			return sources.Observation{}, err
		}
		if err := s.validateAndClearAliases(builder); err != nil {
			return sources.Observation{}, err
		}
		catalog, err = catalogs.NewObservationCatalog(builder)
		if err != nil {
			return sources.Observation{}, err
		}
	}
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

func (s *Source) validateAndClearAliases(builder *catalogs.Builder) error {
	records := builder.CanonicalAliasRecords()
	if len(records) == 0 {
		return nil
	}
	if s.aliasBaseline == nil || !slices.Equal(records, s.aliasBaseline.CanonicalAliasRecords()) {
		return &errors.ConflictError{Resource: "local canonical aliases", Message: "publish rename changes through the selected baseline authority"}
	}
	return builder.SetCanonicalAliasRecords(nil)
}

// Cleanup releases any resources.
func (s *Source) Cleanup() error {
	// LocalSource does not hold any resources
	return nil
}

// Dependencies returns the list of external dependencies.
// Local source has no external dependencies.
func (s *Source) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional reports that a human catalog workspace observation is optional when the
// verified embedded observation is available.
func (s *Source) IsOptional() bool {
	return true
}
