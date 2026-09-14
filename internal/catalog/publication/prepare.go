package publication

import (
	"cmp"
	"context"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	catalogruntime "github.com/agentstation/starmap/runtime"
)

// State retains the selected baseline and accepted observation history.
// A prepared successor does not change this state. The publisher adopts it after promotion succeeds.
type State struct {
	baseline    catalogs.Generation
	current     catalogs.Generation
	publisherID string
	history     []sources.Observation
	bundle      *artifact.Bundle
}

// PreparedPublication contains one admitted candidate, its artifact, and its run receipt.
// The next state stays provisional until the caller adopts the successful publication.
type PreparedPublication struct {
	Bundle         artifact.Bundle
	Receipt        ReceiptRecord
	Decision       Decision
	Next           *State
	ReusedArtifact bool
}

// NewState validates an explicitly trusted baseline without acquiring sources.
// This public-catalog publisher does not accept a permission-bearing enterprise baseline.
func NewState(baseline catalogs.Generation, publisherID string) (*State, error) {
	if baseline.Manifest.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		return nil, admissionError("baseline", "cannot publish an authoritative enterprise catalog")
	}
	if _, err := catalogs.DecodeCatalogGeneration(baseline); err != nil {
		return nil, err
	}
	if _, err := catalogruntime.ReplayAcquisition(context.Background(), baseline, publisherID, nil, nil); err != nil {
		return nil, err
	}
	return &State{baseline: baseline.Copy(), current: baseline.Copy(), publisherID: publisherID}, nil
}

// PublisherID identifies the owner of local source scopes in this checkpoint.
func (s *State) PublisherID() string {
	if s == nil {
		return ""
	}
	return s.publisherID
}

// Generation returns a copy of the accepted catalog generation.
func (s *State) Generation() catalogs.Generation {
	if s == nil {
		return catalogs.Generation{}
	}
	return s.current.Copy()
}

// Prepare collects declared sources and prepares a complete artifact and receipt.
// No branch, channel, accepted state, or external catalog store changes here.
func (p *Producer) Prepare(ctx context.Context, state *State, runID string, opts ...pkgsync.Option) (PreparedPublication, error) {
	return p.prepare(ctx, state, nil, runID, opts...)
}

// PrepareWithBaseline applies an explicitly trusted authored baseline before source acquisition.
// A promoted copy of this publisher's accepted catalog does not replace its original baseline.
// Failed admission leaves the supplied state unchanged.
func (p *Producer) PrepareWithBaseline(ctx context.Context, state *State, baseline catalogs.Generation, runID string, opts ...pkgsync.Option) (PreparedPublication, error) {
	return p.prepare(ctx, state, &baseline, runID, opts...)
}

func (p *Producer) prepare(ctx context.Context, state *State, baseline *catalogs.Generation, runID string, opts ...pkgsync.Option) (PreparedPublication, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return PreparedPublication{}, err
	}
	if p == nil || state == nil || !validRunID(runID) {
		return PreparedPublication{}, admissionError("preparation", "requires a producer, accepted state, and a run identity")
	}
	history, bindings, err := publicationHistory(p.profile, state.history, nil)
	if err != nil {
		return PreparedPublication{}, err
	}
	current, err := catalogs.DecodeCatalogGeneration(state.current)
	if err != nil {
		return PreparedPublication{}, err
	}
	if baseline != nil {
		selected, changed, err := selectPublicationBaseline(state, *baseline)
		if err != nil {
			return PreparedPublication{}, err
		}
		state = selected
		if changed {
			current, err = publicationCollectionBaseline(ctx, state, bindings, history, runID)
			if err != nil {
				return PreparedPublication{}, err
			}
		}
	}
	retained, err := retainedForProfile(p.profile, state.history)
	if err != nil {
		return PreparedPublication{}, err
	}
	collected, err := p.Collect(ctx, current, retained, opts...)
	if err != nil {
		return PreparedPublication{}, err
	}
	return preparePublication(ctx, state, p.profile, collected.Run, runID)
}

func preparePublication(ctx context.Context, state *State, profile Profile, run Run, runID string) (PreparedPublication, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return PreparedPublication{}, err
	}
	if state == nil || !validRunID(runID) {
		return PreparedPublication{}, admissionError("preparation", "requires accepted state and a run identity")
	}
	decision, err := Admit(profile, run)
	if err != nil {
		return PreparedPublication{}, err
	}
	if !decision.Allowed {
		return PreparedPublication{Decision: decision}, admissionError("admission", "required source evidence is unavailable")
	}
	history, bindings, err := publicationHistory(profile, state.history, decision.Inputs)
	if err != nil {
		return PreparedPublication{}, err
	}
	generation, history, err := catalogruntime.PrepareAcquisitionReplay(ctx, state.baseline, state.publisherID, bindings, history, runID, run.CompletedAt)
	if err != nil {
		return PreparedPublication{}, err
	}
	reviewManifest := generation.Manifest.Copy()
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		return PreparedPublication{}, err
	}
	var bundle artifact.Bundle
	reused := false
	if state.bundle != nil {
		previous, err := state.current.SemanticChecksum()
		if err != nil {
			return PreparedPublication{}, err
		}
		if previous == semantic {
			generation = state.current.Copy()
			bundle = cloneBundle(*state.bundle)
			reused = true
		}
	}
	if !reused {
		bundle, err = artifact.Build(generation)
		if err != nil {
			return PreparedPublication{}, err
		}
	}
	receipt, err := BindReceipt(ctx, profile, run, runID, semantic, bundle.Data, bundle.Attestation)
	if err != nil {
		return PreparedPublication{}, err
	}
	receipt, err = bindReviewReceipt(receipt, reviewManifest)
	if err != nil {
		return PreparedPublication{}, err
	}
	if err := ctx.Err(); err != nil {
		return PreparedPublication{}, err
	}
	ownedBundle := cloneBundle(bundle)
	next := &State{baseline: state.baseline.Copy(), current: generation.Copy(), publisherID: state.publisherID, history: history, bundle: &ownedBundle}
	return PreparedPublication{Bundle: bundle, Receipt: receipt, Decision: decision, Next: next, ReusedArtifact: reused}, nil
}

func cloneBundle(bundle artifact.Bundle) artifact.Bundle {
	bundle.Data = slices.Clone(bundle.Data)
	bundle.Attestation = slices.Clone(bundle.Attestation)
	return bundle
}

func retainedForProfile(profile Profile, history []sources.Observation) ([]sources.Observation, error) {
	policies, err := validateProfile(profile)
	if err != nil {
		return nil, err
	}
	latest := make(map[scopeKey]sources.Observation)
	for _, observation := range history {
		scope := Scope{Source: observation.SourceID, Binding: observation.ProviderBinding}
		policy, selected := policies[scope.key()]
		if !selected || !policy.Scope.equal(scope) || !usableEvidence(policy, observation) {
			continue
		}
		prior, found := latest[scope.key()]
		if !found || observation.ObservedAt.After(prior.ObservedAt) {
			latest[scope.key()] = observation
		}
	}
	var retained []sources.Observation
	for _, policy := range profile.Scopes {
		if observation, found := latest[policy.Scope.key()]; found {
			retained = append(retained, cloneObservation(observation))
		}
	}
	return retained, nil
}

func publicationHistory(profile Profile, previous, additions []sources.Observation) ([]sources.Observation, []sources.ProviderAcquisitionBinding, error) {
	policies, err := validateProfile(profile)
	if err != nil {
		return nil, nil, err
	}
	history := make([]sources.Observation, 0, len(previous)+len(additions))
	seen := make(map[string]sources.ObservationReceipt)
	bindings := make(map[string]sources.ProviderAcquisitionBinding)
	for _, input := range [][]sources.Observation{previous, additions} {
		for _, observation := range input {
			scope := Scope{Source: observation.SourceID, Binding: observation.ProviderBinding}
			if policy, found := policies[scope.key()]; found {
				if !policy.Enabled && policy.DisabledAction == Remove {
					continue
				}
				if !scope.equal(policy.Scope) {
					if scope.Binding != nil && policy.Scope.Binding != nil && scope.Binding.Revision == policy.Scope.Binding.Revision {
						return nil, nil, admissionError("history.binding", "selectors changed without a new binding revision")
					}
					continue
				}
			}
			receipt, err := observation.Receipt()
			if err != nil {
				return nil, nil, err
			}
			if prior, found := seen[observation.ID]; found {
				if !reflect.DeepEqual(prior, receipt) {
					return nil, nil, admissionError("history.observation", "one identity has conflicting original evidence")
				}
				continue
			}
			if scope.Binding != nil {
				if old, exists := bindings[scope.Binding.ID]; exists && old != *scope.Binding {
					return nil, nil, admissionError("history.binding", "one binding identity has multiple active declarations")
				}
				bindings[scope.Binding.ID] = *scope.Binding
			}
			seen[observation.ID] = receipt
			history = append(history, cloneObservation(observation))
		}
	}
	slices.SortStableFunc(history, func(a, b sources.Observation) int { return a.ObservedAt.Compare(b.ObservedAt) })
	active := make([]sources.ProviderAcquisitionBinding, 0, len(bindings))
	for _, binding := range bindings {
		active = append(active, binding)
	}
	slices.SortFunc(active, func(a, b sources.ProviderAcquisitionBinding) int { return cmp.Compare(a.ID, b.ID) })
	return history, active, nil
}

const maxPublicationRunIDBytes = 512

func validRunID(value string) bool {
	return value != "" && len(value) <= maxPublicationRunIDBytes && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}
