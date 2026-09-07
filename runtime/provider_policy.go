package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// providerBindingPolicy holds one active declaration per binding identity.
// The map belongs to the runtime and remains immutable after construction.
type providerBindingPolicy struct {
	bindings map[string]sources.ProviderAcquisitionBinding
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

// generationID binds selected declarations and catalog bytes to one opaque identity.
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
