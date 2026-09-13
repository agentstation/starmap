package pipeline

import (
	"context"
	"slices"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Collected holds original source evidence before health policy or reconciliation.
// Publication admission must select valid inputs before it builds a candidate.
type Collected struct {
	StartedAt        time.Time
	CompletedAt      time.Time
	Observations     []sources.Observation
	SourceFailures   []error
	SourceActivities []sources.SourceActivity
	ProviderAttempts []sources.ProviderAttempt
	Options          *pkgsync.Options
	WorkspaceInput   workspace.InputExpectation

	configured []sources.Source
	observeErr error
}

// FailureSummaries identifies typed source failures without exposing diagnostic messages.
func (c *Collected) FailureSummaries() []sources.SourceFailure {
	return sourceFailureSummaries(c.SourceFailures)
}

// Collect observes configured sources without reconciliation or publication.
// It retains source failures for admission and returns cancellation as an error.
// Selection, paths, credentials, and dependency checks use the regular pipeline.
func (p *Pipeline) Collect(ctx context.Context, existing *catalogs.Catalog, opts ...pkgsync.Option) (*Collected, error) {
	options := pkgsync.Defaults().Apply(opts...)
	if p == nil || existing == nil {
		return p.collect(ctx, existing, options)
	}
	ctx, cancel, err := prepareSyncContext(ctx, options.Timeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	tracked, run := p.withSourceActivity(options)
	collected, err := tracked.collect(ctx, existing, options)
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = ctxErr
	}
	if err != nil {
		return nil, &sources.ActivityError{Activities: run.snapshot(), ProviderAttempts: run.providerReport(), Err: err}
	}
	collected.SourceActivities = run.snapshot()
	collected.ProviderAttempts = run.providerReport()
	if collected.observeErr != nil {
		collected.SourceFailures = append(collected.SourceFailures, collected.observeErr)
	}
	collected.configured = nil
	collected.observeErr = nil
	return collected, nil
}

func (p *Pipeline) collect(ctx context.Context, existing *catalogs.Catalog, options *pkgsync.Options) (*Collected, error) {
	if p == nil {
		return nil, &pkgerrors.ValidationError{
			Field:   "pipeline",
			Message: "is required",
		}
	}
	if existing == nil {
		return nil, &pkgerrors.ValidationError{
			Field:   "pipeline.existing_catalog",
			Message: "is required",
		}
	}

	startedAt := time.Now().UTC()
	if err := options.ValidateFilesystemLayout(); err != nil {
		return nil, err
	}
	inputs, err := p.loadCatalogInputs(ctx, options.CatalogPath, existing)
	if err != nil {
		return nil, err
	}
	validationProviders := inputs.providerConfig.Providers()
	acquiresProviders := len(options.Sources) == 0 || slices.Contains(options.Sources, sources.ProvidersID)
	// Metadata filters can name embedded providers without enabling their APIs.
	if !acquiresProviders {
		validationProviders = metadataProviderRegistry(inputs, options.ProviderID)
	}
	if err = options.Validate(validationProviders); err != nil {
		return nil, err
	}

	srcs := p.createSources(options, inputs)
	srcs, err = p.bindProviderSources(srcs, options, inputs)
	if err != nil {
		return nil, err
	}

	srcs, sourceFailures, err := p.resolveDependencies(ctx, srcs, options)
	if err != nil {
		return nil, err
	}

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), constants.SyncCleanupTimeout)
		defer cleanupCancel()

		if cleanupErr := p.cleanup(cleanupCtx, srcs); cleanupErr != nil {
			logging.Warn().Err(cleanupErr).Msg("Source cleanup errors occurred")
		}
	}()

	observations, observeErr := p.observe(ctx, srcs, options.SourceOptions())
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return &Collected{
		StartedAt: startedAt, CompletedAt: time.Now().UTC(),
		Observations: observations, SourceFailures: sourceFailures,
		Options: options, WorkspaceInput: inputs.workspaceInput,
		configured: srcs, observeErr: observeErr,
	}, nil
}
