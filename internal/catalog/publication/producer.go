package publication

import (
	"context"
	"slices"
	"time"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Producer collects declared source scopes and applies publication admission.
// Its immutable profile owns source selection and provider acquisition bindings.
type Producer struct {
	profile  Profile
	selected []sources.ID
	collect  func(context.Context, *catalogs.Catalog, ...pkgsync.Option) (*pipeline.Collected, error)
}

// Collection contains original run evidence and the resulting admission decision.
// These internal values can contain private scope selectors and source diagnostics.
// Public output uses the separate validated publication receipt.
type Collection struct {
	Run      Run      `json:"-"`
	Decision Decision `json:"-"`
}

// NewProducer validates and copies a profile without reading sources or credentials.
func NewProducer(profile Profile, factory sources.ProviderClientFactory, resolver sources.ProviderCredentialResolver) (*Producer, error) {
	if _, err := validateProfile(profile); err != nil {
		return nil, err
	}
	owned := Profile{Version: profile.Version, Scopes: slices.Clone(profile.Scopes)}
	var selected []sources.ID
	var bindings []sources.ProviderAcquisitionBinding
	for i := range owned.Scopes {
		policy := &owned.Scopes[i]
		policy.Scope = policy.Scope.clone()
		if !policy.Enabled {
			continue
		}
		if !slices.Contains(selected, policy.Scope.Source) {
			selected = append(selected, policy.Scope.Source)
		}
		if policy.Scope.Binding != nil {
			bindings = append(bindings, *policy.Scope.Binding)
		}
	}
	if slices.Contains(selected, sources.ModelsDevHTTPID) && slices.Contains(selected, sources.ModelsDevGitID) {
		return nil, admissionError("scopes", "models.dev requires exactly one selected transport")
	}
	collector := pipeline.NewBoundAcquisition(factory, resolver, bindings)
	return &Producer{profile: owned, selected: selected, collect: collector.Collect}, nil
}

// Collect returns original evidence and a decision without building or publishing a catalog.
// The caller must authenticate retained inputs before supplying them.
// Source filters and strict or fresh acquisition options cannot override the profile.
func (p *Producer) Collect(ctx context.Context, current *catalogs.Catalog, retained []sources.Observation, opts ...pkgsync.Option) (Collection, error) {
	if p == nil || p.collect == nil || current == nil {
		return Collection{}, admissionError("producer", "requires a producer and current catalog")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Collection{}, err
	}
	options := pkgsync.Defaults().Apply(opts...)
	if len(options.Sources) != 0 || options.ProviderID != nil || options.Fresh || options.RequireAllSources {
		return Collection{}, admissionError("producer.options", "source selection and evidence requirements belong to the publication profile")
	}
	start := time.Now().UTC()
	run := Run{StartedAt: start, CompletedAt: start}
	for _, policy := range p.profile.Scopes {
		outcome := NotAttempted
		if !policy.Enabled {
			outcome = Disabled
		}
		run.Attempts = append(run.Attempts, Attempt{Scope: policy.Scope.clone(), Outcome: outcome})
	}
	for _, observation := range retained {
		run.Retained = append(run.Retained, cloneObservation(observation))
	}
	if _, err := validateInputs(p.profile, run); err != nil {
		return Collection{}, err
	}
	if len(p.selected) != 0 {
		options.Sources = slices.Clone(p.selected)
		collected, err := p.collect(ctx, current, func(target *pkgsync.Options) { *target = *options })
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Collection{}, ctxErr
		}
		if err != nil {
			collected = collectDependencyFailure(err, start)
			if collected == nil {
				return Collection{}, err
			}
		}
		if collected == nil {
			return Collection{}, admissionError("collection", "collector returned no result")
		}
		run.StartedAt, run.CompletedAt = collected.StartedAt, collected.CompletedAt
		attempts, err := p.collectionAttempts(collected)
		if err != nil {
			return Collection{}, err
		}
		run.Attempts = attempts
	}
	decision, err := Admit(p.profile, run)
	if err != nil {
		return Collection{}, err
	}
	if err := ctx.Err(); err != nil {
		return Collection{}, err
	}
	return Collection{Run: run, Decision: decision}, nil
}
