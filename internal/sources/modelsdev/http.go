package modelsdev

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// HTTPSource enhances models with models.dev data via HTTP.
type HTTPSource struct {
	providers  *catalogs.Providers
	sourcesDir string
	loadAPI    func(context.Context, string) (*API, error)
	acquireAPI func(context.Context, string) (*API, HTTPAcquisitionResult, error)
}

var _ sources.Source = (*HTTPSource)(nil)

// NewHTTPSource creates a new models.dev HTTP source.
func NewHTTPSource(opts ...HTTPSourceOption) *HTTPSource {
	s := &HTTPSource{acquireAPI: acquireHTTPAPI}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// HTTPSourceOption configures an HTTPSource.
type HTTPSourceOption func(*HTTPSource)

// WithHTTPSourcesDir configures the sources directory for the HTTP source.
func WithHTTPSourcesDir(dir string) HTTPSourceOption {
	return func(s *HTTPSource) {
		s.sourcesDir = dir
	}
}

// ID returns the ID of this source.
func (s *HTTPSource) ID() sources.ID {
	return sources.ModelsDevHTTPID
}

// Name returns the human-friendly name of this source.
func (s *HTTPSource) Name() string { return "models.dev (HTTP)" }

func acquireHTTPAPI(ctx context.Context, outputDir string) (*API, HTTPAcquisitionResult, error) {
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultCachePath)
	}
	client := NewHTTPClient(outputDir)
	acquisition, err := client.AcquireAPI(ctx)
	if err != nil {
		return nil, HTTPAcquisitionResult{}, err
	}
	api, err := ParseAPI(client.GetAPIPath())
	return api, acquisition, err
}

// Setup initializes the source with dependencies.
func (s *HTTPSource) Setup(providers *catalogs.Providers) error {
	s.providers = providers
	return nil
}

// Observe returns a catalog with mapped models.dev data directly.
func (s *HTTPSource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	ctx = logging.WithSource(ctx, s.ID().String())
	// Create a new catalog to build into
	builder := catalogs.NewEmpty()

	// Use configured sources directory or default
	outputDir := s.sourcesDir
	if outputDir == "" {
		outputDir = expandPath(constants.DefaultCachePath)
	}

	// Initialize models.dev data once
	var api *API
	var acquisition HTTPAcquisitionResult
	var err error
	if s.loadAPI != nil {
		api, err = s.loadAPI(ctx, outputDir)
		acquisition = HTTPAcquisitionResult{Kind: HTTPAcquisitionDownloaded}
	} else {
		loader := s.acquireAPI
		if loader == nil {
			loader = acquireHTTPAPI
		}
		api, acquisition, err = loader(ctx, outputDir)
	}
	if err != nil {
		return sources.Observation{}, errors.WrapResource("initialize", "models.dev HTTP", "", err)
	}

	// Process the API data using shared logic
	added, rejected, recordIssues, err := processFetch(builder, api, opts...)
	if err != nil {
		return sources.Observation{}, err
	}

	catalog, err := builder.Build()
	if err != nil {
		return sources.Observation{}, errors.WrapResource("publish", "models.dev HTTP observation", "", err)
	}

	logging.FromContext(ctx).Info().
		Int("model_count", added).
		Msg("Found models with catalog data from models.dev HTTP")
	metadata := sources.ObservationMetadata{
		ObservedAt:   time.Now().UTC(),
		Revision:     acquisition.Revision,
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: added, Rejected: rejected},
		Issues:       append([]sources.ObservationIssue(nil), acquisition.Issues...),
	}
	if len(acquisition.Issues) > 0 {
		metadata.Completeness = sources.ObservationCompletenessPartial
		metadata.Status = sources.ObservationStatusDegraded
	}
	if metadata.Revision.Kind == "" {
		metadata.Revision = sources.Revision{Kind: sources.RevisionKindContentDigest}
	}
	switch acquisition.Kind {
	case HTTPAcquisitionStaleCache:
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Issues = append(metadata.Issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback,
			Message: "upstream HTTP acquisition failed; using stale last-known-good cache",
		})
	case HTTPAcquisitionEmbeddedBootstrap:
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Issues = append(metadata.Issues, sources.ObservationIssue{
			Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeBootstrapFallback,
			Message: "upstream HTTP acquisition and cache were unavailable; using embedded bootstrap",
		})
	}
	if len(recordIssues) > 0 {
		metadata.Completeness = sources.ObservationCompletenessPartial
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Issues = append(metadata.Issues, recordIssues...)
	}
	return sources.NewObservation(s.ID(), catalog, metadata)
}

// Cleanup releases any resources.
func (s *HTTPSource) Cleanup() error {
	// HTTPSource doesn't hold persistent resources
	return nil
}

// Dependencies returns the list of external dependencies.
// HTTP source has no external dependencies.
func (s *HTTPSource) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional returns whether this source is optional.
// HTTP source is optional - git source provides same data, and we can work without models.dev.
func (s *HTTPSource) IsOptional() bool {
	return true
}
