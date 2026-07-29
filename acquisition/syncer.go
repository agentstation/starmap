// Package acquisition composes provider and external-source acquisition above
// the small immutable Starmap client.
package acquisition

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Option configures one acquisition Syncer.
type Option func(*options) error

type options struct {
	providerClientFactory sources.ProviderClientFactory
}

func defaults() options {
	return options{providerClientFactory: defaultProviderClientFactory}
}

func defaultProviderClientFactory(
	provider *catalogs.Provider,
) (sources.ProviderClient, error) {
	return clients.NewProvider(provider)
}

// WithProviderClientFactory replaces the concrete provider-client composition.
// It is intended for source plugins, restricted deployments, and deterministic
// integration tests.
func WithProviderClientFactory(factory sources.ProviderClientFactory) Option {
	return func(options *options) error {
		if factory == nil {
			return &errors.ValidationError{
				Field:   "acquisition.provider_client_factory",
				Message: "is required",
			}
		}
		options.providerClientFactory = factory
		return nil
	}
}

// Syncer observes configured sources, reconciles a complete candidate, and
// delegates serialized durable publication to a Client.
type Syncer struct {
	client   *starmap.Client
	pipeline *pipeline.Pipeline
}

// New constructs an explicit acquisition composition. It starts no goroutine
// and performs no source or filesystem work.
func New(client *starmap.Client, opts ...Option) (*Syncer, error) {
	if client == nil {
		return nil, &errors.ValidationError{
			Field:   "acquisition.client",
			Message: "is required",
		}
	}
	config := defaults()
	for _, opt := range opts {
		if opt == nil {
			return nil, &errors.ValidationError{
				Field:   "acquisition.option",
				Message: "cannot be nil",
			}
		}
		if err := opt(&config); err != nil {
			return nil, err
		}
	}
	return &Syncer{
		client:   client,
		pipeline: pipeline.NewAcquisition(config.providerClientFactory),
	}, nil
}

// Sync observes and reconciles sources. Dry runs require no writable store.
// Non-dry runs build inside Client.Update so candidate construction, store CAS,
// and atomic publication remain one serialized mutation transaction.
func (s *Syncer) Sync(
	ctx context.Context,
	opts ...pkgsync.Option,
) (*pkgsync.Result, error) {
	if s == nil || s.client == nil || s.pipeline == nil {
		return nil, &errors.ValidationError{
			Field:   "acquisition.syncer",
			Message: "is required",
		}
	}

	effective, parsed, err := s.effectiveOptions(opts)
	if err != nil {
		return nil, err
	}
	ctx, cancel, err := prepareSyncContext(ctx, parsed.Timeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	if parsed.DryRun {
		prepared, err := s.pipeline.Prepare(ctx, s.client.Catalog(), effective...)
		if err != nil {
			return nil, err
		}
		return prepared.Result, nil
	}

	var prepared *pipeline.Prepared
	var candidateCatalog *catalogs.Catalog
	publication, err := s.client.Update(ctx, func(
		updateCtx context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		var prepareErr error
		prepared, prepareErr = s.pipeline.Prepare(updateCtx, current, effective...)
		if prepareErr != nil {
			return nil, prepareErr
		}
		if !prepared.Publish {
			return nil, nil
		}
		catalog, buildErr := prepared.Catalog.Build()
		if buildErr != nil {
			return nil, errors.WrapResource("publish", "catalog candidate", "", buildErr)
		}
		candidateCatalog = catalog
		links := make([]catalogs.SourceObservationLink, 0, len(prepared.Observations))
		for _, observation := range prepared.Observations {
			links = append(links, observation.Link())
		}
		return starmap.NewCandidate(catalog, links...)
	})
	if err != nil {
		return nil, err
	}
	if prepared == nil {
		return nil, &errors.ValidationError{
			Field:   "acquisition.result",
			Message: "candidate preparation did not complete",
		}
	}

	result := prepared.Result
	if publication.Published {
		result.GenerationID = publication.GenerationID
		result.SyncRunID = publication.SyncRunID
		if prepared.Options.CatalogPath != "" {
			result.Projection = projectCommittedCatalog(
				ctx,
				candidateCatalog,
				prepared.Options.CatalogPath,
				publication,
				prepared.WorkspaceInput,
			)
		}
		logging.Info().
			Int("changes_applied", result.TotalChanges).
			Msg("Sync completed successfully")
	}
	return result, nil
}

func prepareSyncContext(
	ctx context.Context,
	timeout time.Duration,
) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logging.RunID(ctx) == "" {
		runID, err := uuid.NewRandom()
		if err != nil {
			return nil, nil, errors.WrapResource("generate", "source run ID", "", err)
		}
		ctx = logging.WithRunID(ctx, "source-run-"+runID.String())
	}
	if timeout <= 0 {
		return ctx, func() {}, nil
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	return bounded, cancel, nil
}

func (s *Syncer) effectiveOptions(
	opts []pkgsync.Option,
) ([]pkgsync.Option, *pkgsync.Options, error) {
	parsed := pkgsync.Defaults().Apply(opts...)
	configuredPath := s.client.WorkspacePath()
	if parsed.CatalogPath != "" && parsed.CatalogPath != configuredPath {
		return nil, nil, &errors.ConfigError{
			Component: "acquisition",
			Message: "sync catalog path is not the Client workspace; " +
				"construct the Client with starmap.WithCatalogPath",
		}
	}

	effective := append([]pkgsync.Option(nil), opts...)
	if parsed.CatalogPath == "" && configuredPath != "" {
		effective = append(effective, pkgsync.WithCatalogPath(configuredPath))
		parsed = pkgsync.Defaults().Apply(effective...)
	}
	return effective, parsed, nil
}
