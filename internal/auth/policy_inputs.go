package auth

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type environmentSelection struct {
	value   string
	present bool
}

type policyInputs struct {
	base        *Resolver
	environment map[string]environmentSelection
	sources     map[ReferenceBackend]credentialSource
	chains      map[catalogs.ProviderAuthenticationPrimitive]cloudChain
}

// policyInputs shares selected source responses between the two policy evaluations.
func newPolicyInputs(base *Resolver, provider catalogs.ProviderID) *policyInputs {
	inputs := &policyInputs{
		base:        base,
		environment: make(map[string]environmentSelection),
		sources:     make(map[ReferenceBackend]credentialSource),
		chains:      make(map[catalogs.ProviderAuthenticationPrimitive]cloudChain),
	}
	for backend, source := range base.sources {
		if backend == referenceBackendEnvironment {
			source = environmentSource{lookup: inputs.lookup}
		}
		inputs.sources[backend] = &policyInputSource{base: base, provider: provider, source: source, results: make(map[Reference]policySourceResult)}
	}
	for primitive, chain := range base.cloudChains {
		inputs.chains[primitive] = &policyInputChain{base: base, provider: provider, chain: chain, results: make(map[catalogs.ProviderCredentialProfileID]policySourceResult)}
	}
	return inputs
}

func (i *policyInputs) lookup(name string) (string, bool) {
	if selected, exists := i.environment[name]; exists {
		return selected.value, selected.present
	}
	value, present := i.base.lookup(name)
	i.environment[name] = environmentSelection{value: value, present: present}
	return value, present
}

func (i *policyInputs) resolver(policy EnvironmentPolicy) *Resolver {
	resolver := newResolver(i.lookup, WithEnvironmentPolicy(policy), WithReferencePolicies(i.base.references))
	resolver.sources = i.sources
	resolver.cloudChains = i.chains
	resolver.versionSeed = i.base.versionSeed
	return resolver
}

type policySourceResult struct {
	material sourceMaterial
	err      error
}

type policyInputSource struct {
	base     *Resolver
	provider catalogs.ProviderID
	source   credentialSource
	results  map[Reference]policySourceResult
}

func (s *policyInputSource) Backend() ReferenceBackend { return s.source.Backend() }

func (s *policyInputSource) Resolve(ctx context.Context, reference Reference) (sourceMaterial, error) {
	if result, exists := s.results[reference]; exists {
		return result.material.copy(), result.err
	}
	for previous, result := range s.results {
		if previous.resource == reference.resource && previous.version == reference.version && result.err == nil && result.material.snapshot != nil {
			return result.material.copy(), nil
		}
	}
	identity := s.base.opaqueVersion("policy-reference", string(s.provider), string(reference.backend), reference.resource, reference.field, reference.version)
	material, err := s.base.resolveSource(ctx, identity, func(ctx context.Context) (sourceMaterial, error) {
		return s.source.Resolve(ctx, reference)
	})
	s.results[reference] = policySourceResult{material: material.copy(), err: err}
	return material, err
}

type policyInputChain struct {
	base     *Resolver
	provider catalogs.ProviderID
	chain    cloudChain
	results  map[catalogs.ProviderCredentialProfileID]policySourceResult
}

func (c *policyInputChain) resolve(ctx context.Context, profile catalogs.ProviderCredentialProfile, fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField) (sourceMaterial, error) {
	if result, exists := c.results[profile.ID]; exists {
		return result.material.copy(), result.err
	}
	identity := c.base.opaqueVersion("cloud-chain", string(c.provider), string(profile.ID), string(profile.Primitive))
	material, err := c.base.resolveSource(ctx, identity, func(ctx context.Context) (sourceMaterial, error) {
		return c.chain.resolve(ctx, profile, fields)
	})
	c.results[profile.ID] = policySourceResult{material: material.copy(), err: err}
	return material, err
}
