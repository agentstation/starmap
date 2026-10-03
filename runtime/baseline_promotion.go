package runtime

import (
	"context"
	"maps"
	"slices"
	"strconv"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// baselinePromotionResource names the operation in promotion conflicts.
const baselinePromotionResource = "fleet baseline promotion"

// BaselineStatus compares this binary's packaged baseline with the fleet's retained baseline.
type BaselineStatus struct {
	// Packaged identifies the embedded baseline compiled into this binary.
	Packaged catalogs.GenerationIdentity
	// Retained identifies the baseline that the accepted fleet publication replays.
	Retained catalogs.GenerationIdentity
	// Head is the accepted fleet publication that this replica observed.
	Head FleetHead
	// Promotable reports whether PromoteEmbeddedBaseline can adopt Packaged at Head.
	// It does not establish lease ownership.
	Promotable bool
	// Refusal explains why promotion is unavailable. It is empty when Promotable is true.
	Refusal string
}

// BaselineStatus returns the packaged and retained baselines without a storage operation.
// It returns false when the runtime has no fleet store.
func (r *Runtime) BaselineStatus() (BaselineStatus, bool) {
	if r == nil || r.config.fleetStore == nil {
		return BaselineStatus{}, false
	}
	packaged := generationIdentity(r.client.EmbeddedCatalogState())
	r.mu.RLock()
	defer r.mu.RUnlock()
	status := BaselineStatus{Packaged: packaged, Retained: generationIdentity(r.layers.embedded), Head: r.fleetHead}
	if refusal := r.baselinePromotionRefusalLocked(packaged); refusal != nil {
		status.Refusal = refusal.Message
	}
	status.Promotable = status.Refusal == ""
	return status, true
}

// BaselinePromotion selects the packaged baseline at one observed fleet head.
type BaselinePromotion struct {
	// ExpectedHead is the accepted publication that the operator reviewed.
	ExpectedHead FleetHead
	// PackagedGenerationID must name the embedded baseline of this binary.
	PackagedGenerationID string
}

// BaselinePromotionResult describes one accepted baseline promotion.
type BaselinePromotionResult struct {
	// Head is the fleet publication that retains the promoted baseline.
	Head FleetHead
	// Previous identifies the retained baseline before promotion.
	Previous catalogs.GenerationIdentity
	// Promoted identifies the packaged baseline that the fleet now retains.
	Promoted catalogs.GenerationIdentity
	// InertRemovals lists retained removal targets that match nothing in the promoted catalog.
	// They stay recorded until an explicit removal edit replaces them.
	InertRemovals []catalogs.CatalogRemovalTarget
}

// PromoteEmbeddedBaseline makes this binary's packaged baseline the fleet's retained baseline.
// It runs under the publication lease and publishes one revision after ExpectedHead.
// Retained source, provider, manual, and removal inputs stay unchanged.
// It refuses a stale head, a different packaged generation, an equal baseline, a generation pin, and a configured source authority.
// The caller authorizes the operator action.
func (r *Runtime) PromoteEmbeddedBaseline(ctx context.Context, request BaselinePromotion) (BaselinePromotionResult, error) {
	if request.PackagedGenerationID == "" {
		return BaselinePromotionResult{}, &errors.ValidationError{Field: "promotion.packaged_generation_id", Message: "is required"}
	}
	if request.ExpectedHead == (FleetHead{}) {
		return BaselinePromotionResult{}, &errors.ValidationError{Field: "promotion.expected_head", Message: "is required"}
	}
	if err := request.ExpectedHead.Validate(); err != nil {
		return BaselinePromotionResult{}, err
	}
	if r == nil || r.config.fleetStore == nil {
		return BaselinePromotionResult{}, baselinePromotionConflict("only a fleet runtime retains a baseline independently of its binary")
	}
	packaged := generationIdentity(r.client.EmbeddedCatalogState())
	r.mu.RLock()
	refusal := r.baselinePromotionRefusalLocked(packaged)
	r.mu.RUnlock()
	if refusal != nil {
		return BaselinePromotionResult{}, refusal
	}
	var result BaselinePromotionResult
	_, err := r.execute(ctx, runKindManual, func(runCtx context.Context, _ *RefreshReport, epoch uint64) error {
		var err error
		result, err = r.promoteEmbeddedBaseline(runCtx, request, epoch)
		return err
	})
	return result, err
}

func (r *Runtime) promoteEmbeddedBaseline(ctx context.Context, request BaselinePromotion, epoch uint64) (BaselinePromotionResult, error) {
	baseline, err := bootstrap.Generation()
	if err != nil {
		return BaselinePromotionResult{}, err
	}
	embedded := r.client.EmbeddedCatalogState()
	packaged := generationIdentity(embedded)
	if baseline.Manifest.GenerationID != packaged.GenerationID || baseline.Manifest.Payload.Checksum != packaged.PayloadChecksum {
		return BaselinePromotionResult{}, baselinePromotionConflict("the packaged generation differs from the client's embedded catalog")
	}
	r.publicationMu.Lock()
	defer r.publicationMu.Unlock()
	r.providerRetentionMu.Lock()
	defer r.providerRetentionMu.Unlock()
	if err := r.lease.fence(epoch); err != nil {
		return BaselinePromotionResult{}, err
	}
	r.mu.RLock()
	head := r.fleetHead
	refusal := r.baselinePromotionRefusalLocked(packaged)
	candidate := r.layers
	candidate.providers = maps.Clone(r.layers.providers)
	r.mu.RUnlock()
	if head != request.ExpectedHead {
		return BaselinePromotionResult{}, &errors.ConflictError{Resource: baselinePromotionResource,
			Expected: strconv.FormatUint(request.ExpectedHead.Revision, 10), Actual: strconv.FormatUint(head.Revision, 10),
			Message: "the fleet head changed before promotion"}
	}
	if request.PackagedGenerationID != packaged.GenerationID {
		return BaselinePromotionResult{}, &errors.ConflictError{Resource: baselinePromotionResource,
			Expected: request.PackagedGenerationID, Actual: packaged.GenerationID,
			Message: "the request names a different packaged generation than this binary"}
	}
	if refusal != nil {
		return BaselinePromotionResult{}, refusal
	}
	previous := generationIdentity(candidate.embedded)
	candidate.embedded, candidate.embeddedManifest, candidate.fleetBaseline = embedded, &baseline.Manifest, &baseline
	inert, err := candidate.inertRemovalTargets(ctx)
	if err != nil {
		return BaselinePromotionResult{}, err
	}
	state, err := candidate.build(ctx, candidate.embedded)
	if err != nil {
		return BaselinePromotionResult{}, err
	}
	publication, err := r.preparePublication(ctx, state, candidate.buildEvidence, candidate.source)
	if err != nil {
		return BaselinePromotionResult{}, err
	}
	durable, err := r.commitPrepared(ctx, publication, epoch, candidate.buildEvidence, candidate.source, candidate)
	if err != nil {
		return BaselinePromotionResult{}, err
	}
	r.mu.Lock()
	r.layers = candidate
	r.effective = durable
	r.activateAuthorityLocked(candidate.source)
	head = r.fleetHead
	r.mu.Unlock()
	r.broadcast(durable)
	return BaselinePromotionResult{Head: head, Previous: previous, Promoted: packaged, InertRemovals: inert}, nil
}

// baselinePromotionRefusalLocked reports a refusal that holds at any head. The caller holds r.mu.
func (r *Runtime) baselinePromotionRefusalLocked(packaged catalogs.GenerationIdentity) *errors.ConflictError {
	switch {
	case r.config.generationPin != "" || r.pinRecord != nil:
		return baselinePromotionConflict("clear the generation pin before promoting the baseline")
	case r.requiresAuthority() || r.config.origin != nil:
		return baselinePromotionConflict("a configured source authority owns the catalog baseline")
	case r.fleetReplayError != nil:
		return baselinePromotionConflict("this replica cannot replay the accepted fleet inputs")
	case r.fleetHead == (FleetHead{}):
		return baselinePromotionConflict("the fleet has no accepted publication")
	case packaged == generationIdentity(r.layers.embedded):
		return baselinePromotionConflict("the packaged baseline is already the retained baseline")
	}
	return nil
}

func baselinePromotionConflict(message string) *errors.ConflictError {
	return &errors.ConflictError{Resource: baselinePromotionResource, Message: message}
}

func generationIdentity(state starmap.CatalogState) catalogs.GenerationIdentity {
	return catalogs.GenerationIdentity{GenerationID: state.GenerationID, PayloadChecksum: state.PayloadChecksum}
}

// inertRemovalTargets lists retained targets that match nothing in the catalog that these inputs build.
// A copy builds without the removal policy, so the receiver keeps its evidence and sequence.
func (l *layerSet) inertRemovalTargets(ctx context.Context) ([]catalogs.CatalogRemovalTarget, error) {
	if l.removals == nil || len(l.removals.Targets) == 0 {
		return nil, nil
	}
	targets := l.removals.Targets
	probe := *l
	probe.removals = nil
	state, err := probe.build(ctx, probe.embedded)
	if err != nil {
		return nil, err
	}
	scopes := state.Catalog.MembershipScopes()
	var inert []catalogs.CatalogRemovalTarget
	for _, target := range targets {
		if !removalTargetPresent(state.Catalog, scopes, target) {
			inert = append(inert, target)
		}
	}
	return inert, nil
}

// removalTargetPresent uses the presence rules of an operator removal edit.
func removalTargetPresent(catalog *catalogs.Catalog, scopes []catalogs.ProviderMembershipScope, target catalogs.CatalogRemovalTarget) bool {
	switch target.Kind {
	case catalogs.CatalogRemovalAlias:
		_, _, found := catalog.CanonicalAliases().Lookup(target.AliasID)
		return found
	case catalogs.CatalogRemovalCanonical:
		_, err := catalog.Definition(target.DefinitionID)
		return err == nil
	case catalogs.CatalogRemovalScoped:
		if target.Scope == nil || !slices.ContainsFunc(scopes, func(scope catalogs.ProviderMembershipScope) bool {
			return target.MatchesScope(scope, target.ProviderModelID)
		}) {
			return false
		}
		_, err := catalog.Offering(target.Scope.ProviderID, target.ProviderModelID)
		return err == nil
	}
	return false
}
