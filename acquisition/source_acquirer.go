package acquisition

import (
	"context"
	stderrors "errors"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

// SourceAcquirer uses the shared registered sources for connected acquisition.
// Provider APIs remain with Acquirer so their publication windows stay independent.
type SourceAcquirer struct {
	options  pkgsync.Options
	pipeline *pipeline.Pipeline
}

// NewSourceAcquirer prepares non-provider acquisition without source reads.
// WithSources selects local or one models.dev form. The default uses local and HTTP.
// Other sync options use the same path, dependency, and timeout semantics as manual acquisition.
func NewSourceAcquirer(opts ...pkgsync.Option) (*SourceAcquirer, error) {
	options := pkgsync.Defaults().Apply(opts...)
	if len(options.Sources) == 0 {
		options.Sources = []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID}
	}
	for _, id := range options.Sources {
		if id != sources.LocalCatalogID && id != sources.ModelsDevHTTPID && id != sources.ModelsDevGitID {
			return nil, &errors.ValidationError{Field: "source_acquirer.sources", Message: "must select local or models.dev acquisition sources"}
		}
	}
	if slices.Contains(options.Sources, sources.ModelsDevHTTPID) && slices.Contains(options.Sources, sources.ModelsDevGitID) {
		return nil, &errors.ValidationError{Field: "source_acquirer.sources", Message: "must select only one models.dev source form"}
	}
	if options.Fresh || options.DryRun || options.ProviderID != nil {
		return nil, &errors.ValidationError{Field: "source_acquirer.options", Message: "fresh, dry-run, and fixed provider selection require explicit manual acquisition"}
	}
	options.Sources = slices.Clone(options.Sources)
	return &SourceAcquirer{options: *options, pipeline: pipeline.NewAcquisition(nil, nil)}, nil
}

// AcquireSources returns original observations without publishing a catalog.
// Provider filters use the shared pipeline contract. A failed scope preserves other results.
func (a *SourceAcquirer) AcquireSources(ctx context.Context, request runtime.SourceAcquisitionRequest) ([]sources.Observation, error) {
	if a == nil || a.pipeline == nil {
		return nil, &errors.ValidationError{Field: "source_acquirer", Message: "is required"}
	}
	if request.Current == nil {
		return nil, &errors.ValidationError{Field: "source_acquirer.current", Message: "is required"}
	}
	ctx, cancel, err := prepareSyncContext(ctx, a.options.Timeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	for _, provider := range request.Providers {
		if strings.TrimSpace(string(provider)) == "" {
			return nil, &errors.ValidationError{Field: "source_acquirer.providers", Message: "provider IDs must not be empty"}
		}
	}
	targets := slices.Clone(request.Providers)
	slices.Sort(targets)
	targets = slices.Compact(targets)
	if len(targets) == 0 {
		targets = []catalogs.ProviderID{""}
	}
	var observations []sources.Observation
	var failures []error
	for _, provider := range targets {
		options := a.options
		options.Sources = slices.Clone(a.options.Sources)
		if request.Sources != nil {
			options.Sources = slices.Clone(request.Sources)
		}
		if len(options.Sources) == 0 {
			continue
		}
		if request.ModelsDevGitCommit != nil {
			options.ModelsDevGitCommit = *request.ModelsDevGitCommit
		}
		if !slices.Contains(options.Sources, sources.ModelsDevGitID) {
			options.ModelsDevGitCommit = ""
		}
		if err := sources.ValidateAcquisitionSelection(options.Sources); err != nil {
			return nil, err
		}
		if slices.Contains(options.Sources, sources.ProvidersID) {
			return nil, &errors.ValidationError{Field: "source_acquirer.sources", Message: "provider acquisition uses a separate role"}
		}
		if provider != "" {
			options.ProviderID = &provider
		}
		prepared, err := a.pipeline.Prepare(ctx, request.Current, func(target *pkgsync.Options) { *target = options })
		if err != nil {
			failures = append(failures, err)
			continue
		}
		failures = append(failures, prepared.SourceFailures...)
		reported := make(map[sources.ID]bool, len(prepared.Observations))
		for _, failure := range prepared.Result.SourceFailures {
			reported[failure.Source] = true
		}
		for _, observation := range prepared.Observations {
			reported[observation.SourceID] = true
			if observation.SourceID != sources.EmbeddedCatalogID && observation.SourceID != sources.ReleaseArtifactID {
				observations = append(observations, observation)
			}
		}
		for _, id := range options.Sources {
			if !reported[id] && (id != sources.LocalCatalogID || prepared.WorkspaceInput.Exists) {
				failures = append(failures, &errors.ConfigError{Component: "source acquisition", Message: "selected source " + string(id) + " produced no observation; check its configuration and dependencies"})
			}
		}
	}
	return observations, stderrors.Join(failures...)
}
