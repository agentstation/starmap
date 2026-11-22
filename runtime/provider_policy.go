package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// BindingAcquirer observes only the supplied active declarations.
// It must preserve each declaration from credential selection through its receipt.
// Implementations also implement Acquirer for runtime injection.
type BindingAcquirer interface {
	AcquireProviderBindings(context.Context, AcquisitionRequest, []sources.ProviderAcquisitionBinding) (AcquisitionResult, error)
}

// providerBindingPolicy holds one active declaration per binding identity.
// The map belongs to the runtime and remains immutable after construction.
type providerBindingPolicy struct {
	bindings map[string]sources.ProviderAcquisitionBinding
}

// permitsProjectedEvidence checks a carried provider fact against the active binding set.
func (p *providerBindingPolicy) permitsProjectedEvidence(providerID catalogs.ProviderID, entry provenance.Entry) bool {
	if entry.Source != sources.ProvidersID {
		return true
	}
	if p == nil {
		return entry.ProviderBindingID == "" && entry.ProviderBindingRevision == ""
	}
	active, exists := p.bindings[entry.ProviderBindingID]
	return exists && active.ProviderID == providerID && active.Revision == entry.ProviderBindingRevision
}

// WithProviderBindings selects the complete active set for local provider evidence.
// An empty set permits no local provider evidence. The set excludes unscoped evidence.
// The runtime copies the declarations. A changed set requires a new runtime.
func WithProviderBindings(bindings ...sources.ProviderAcquisitionBinding) Option {
	owned := append([]sources.ProviderAcquisitionBinding(nil), bindings...)
	return func(config *options) error {
		policy := &providerBindingPolicy{bindings: make(map[string]sources.ProviderAcquisitionBinding, len(owned))}
		for _, binding := range owned {
			if err := binding.Validate(); err != nil {
				return err
			}
			if _, exists := policy.bindings[binding.ID]; exists {
				return &errors.ValidationError{Field: "provider_bindings", Message: "each binding identity must have exactly one active declaration"}
			}
			policy.bindings[binding.ID] = binding
		}
		config.providerBindings = policy
		return nil
	}
}

// permits requires an exact active declaration when a policy is explicit.
// Without a policy, existing unscoped acquisition remains the legacy behavior.
func (p *providerBindingPolicy) permits(layer ProviderLayer) bool {
	binding := layer.Receipt.ProviderBinding
	if p == nil {
		return binding == nil
	}
	if binding == nil {
		return false
	}
	active, exists := p.bindings[binding.ID]
	return exists && active == *binding
}

func (p *providerBindingPolicy) selected(providers []catalogs.ProviderID) ([]sources.ProviderAcquisitionBinding, error) {
	selected := make([]sources.ProviderAcquisitionBinding, 0, len(p.bindings))
	matched := make(map[catalogs.ProviderID]bool, len(providers))
	for _, binding := range p.bindings {
		if len(providers) == 0 || slices.Contains(providers, binding.ProviderID) {
			selected = append(selected, binding)
			matched[binding.ProviderID] = true
		}
	}
	for _, provider := range providers {
		if !matched[provider] {
			return nil, &errors.ValidationError{Field: "provider_bindings.providers", Message: "requested provider has no active binding"}
		}
	}
	slices.SortFunc(selected, func(left, right sources.ProviderAcquisitionBinding) int { return strings.Compare(left.ID, right.ID) })
	return selected, nil
}

func (p *providerBindingPolicy) validateRetained(layers map[providerEvidenceKey]ProviderLayer) error {
	if p == nil {
		return nil
	}
	for _, layer := range layers {
		binding := layer.Receipt.ProviderBinding
		if binding == nil {
			continue
		}
		if active, exists := p.bindings[binding.ID]; exists && active.Revision == binding.Revision && active != *binding {
			return &errors.ConflictError{Resource: "provider binding revision", Message: "active selectors differ from retained evidence without a new revision"}
		}
	}
	return nil
}

func (p *providerBindingPolicy) validatePublication(layers []ProviderLayer) error {
	for _, layer := range layers {
		if !p.permits(layer) {
			return &errors.ConflictError{Resource: "provider evidence", Message: "observation has no matching active binding declaration"}
		}
	}
	return nil
}

// generationID binds selected declarations and a content checksum to one opaque identity.
func (p *providerBindingPolicy) generationID(upstream, checksum string) (string, error) {
	bindings := make([]sources.ProviderAcquisitionBinding, 0, len(p.bindings))
	for _, binding := range p.bindings {
		bindings = append(bindings, binding)
	}
	slices.SortFunc(bindings, func(left, right sources.ProviderAcquisitionBinding) int { return strings.Compare(left.ID, right.ID) })
	raw, err := json.Marshal(struct {
		Domain   string
		Upstream string
		Checksum string
		Bindings []sources.ProviderAcquisitionBinding
	}{"starmap-binding-generation:v1", upstream, checksum, bindings})
	if err != nil {
		return "", errors.WrapResource("encode", "active provider bindings", "", err)
	}
	digest := sha256.Sum256(raw)
	return "bindings-" + hex.EncodeToString(digest[:]), nil
}

// acquireSelectedProviders prevents an explicit policy from invoking an unscoped role.
func (r *Runtime) acquireSelectedProviders(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, bool, error) {
	if r.config.providerBindings == nil {
		result, err := r.config.acquirer.AcquireProviders(ctx, request)
		return result, true, err
	}
	bindings, err := r.config.providerBindings.selected(request.Providers)
	if err != nil {
		return AcquisitionResult{}, false, err
	}
	if len(bindings) == 0 {
		return AcquisitionResult{}, false, nil
	}
	acquirer, ok := r.config.acquirer.(BindingAcquirer)
	if !ok {
		return AcquisitionResult{}, false, &errors.ConfigError{Component: "binding acquisition", Message: "acquirer must support active provider bindings"}
	}
	result, err := acquirer.AcquireProviderBindings(ctx, request, bindings)
	return result, true, err
}

// storedProviderPolicyRequired identifies local provider facts with a recorded binding scope.
// The exact compiled baseline carries publisher evidence independent of local declarations.
func storedProviderPolicyRequired(state, baseline starmap.CatalogState) bool {
	if baseline.Catalog != nil && baseline.GenerationID != "" && baseline.PayloadChecksum != "" &&
		state.GenerationID == baseline.GenerationID && state.PayloadChecksum == baseline.PayloadChecksum &&
		state.GeneratedAt.Equal(baseline.GeneratedAt) && state.AuthorityHead == baseline.AuthorityHead {
		return false
	}
	if state.Catalog == nil {
		return false
	}
	for _, entries := range state.Catalog.Provenance().Map() {
		for _, entry := range entries {
			if entry.Source == sources.ProvidersID && (entry.ProviderBindingID != "" || entry.ProviderBindingRevision != "") {
				return true
			}
		}
	}
	return false
}

// storedOriginMatchesBaseline checks whether the configured origin published the compiled baseline.
// Local provider acquisition still requires explicit bindings or retained input recovery.
func (r *Runtime) storedOriginMatchesBaseline(ctx context.Context, baseline starmap.CatalogState) (bool, error) {
	if r.config.origin == nil || baseline.Catalog == nil || baseline.GenerationID == "" {
		return false, nil
	}
	generation, err := r.client.CurrentGeneration(ctx)
	if err != nil {
		return false, err
	}
	if err := r.config.origin.validateCurrent(generation); err != nil {
		return false, err
	}
	return r.config.origin.matchesSource(generation, baseline)
}

// providerInventoryOmitsRecords preserves original evidence for omitted offerings.
// A complete inventory changes observed availability without deleting visible records.
func (l *layerSet) providerInventoryOmitsRecords(ctx context.Context, incoming []ProviderLayer) (bool, error) {
	for _, layer := range incoming {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		prior, exists := l.providers[layer.evidenceKey()]
		if !exists {
			continue
		}
		previous, err := catalogs.DecodeSourceObservationPayload(prior.Payload)
		if err != nil {
			return false, err
		}
		next, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
		if err != nil {
			return false, err
		}
		oldProvider, oldExists := previous.Providers().Get(layer.ProviderID)
		newProvider, newExists := next.Providers().Get(layer.ProviderID)
		if !oldExists || !newExists {
			return false, invalidProviderEvidence("provider_id", "inventory does not contain the declared provider")
		}
		for modelID := range oldProvider.Models {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			if _, exists := newProvider.Models[modelID]; !exists {
				return true, nil
			}
		}
	}
	return false, nil
}
