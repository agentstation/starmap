package auth

import (
	"context"
	"fmt"
	"hash/maphash"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const starmapCredentialProduct = "STARMAP"

type environmentLookup func(string) (string, bool)

type profileResolver func(
	context.Context,
	*catalogs.Provider,
	catalogs.ProviderCredentialProfile,
	map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error)

type cloudChain interface {
	resolve(
		context.Context,
		catalogs.ProviderCredentialProfile,
		map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
	) (sourceMaterial, error)
}

type sourceResolve func(context.Context) (sourceMaterial, error)

type resolutionCall struct {
	done     chan struct{}
	material sourceMaterial
	err      error
}

// Resolver selects catalog-declared authentication primitives and resolves
// deployment-owned credential sources. It contains no provider roster.
type Resolver struct {
	lookup      environmentLookup
	handlers    map[catalogs.ProviderAuthenticationPrimitive]profileResolver
	references  map[CredentialFieldKey]ReferencePolicy
	sources     map[ReferenceBackend]credentialSource
	cloudChains map[catalogs.ProviderAuthenticationPrimitive]cloudChain
	versionSeed maphash.Seed

	mu       sync.Mutex
	cache    map[string]sourceMaterial
	inflight map[string]*resolutionCall
}

// NewResolver creates the built-in catalog credential resolver.
func NewResolver(options ...ResolverOption) *Resolver {
	return newResolver(os.LookupEnv, options...)
}

func newResolver(lookup environmentLookup, options ...ResolverOption) *Resolver {
	resolver := &Resolver{
		lookup:      lookup,
		references:  make(map[CredentialFieldKey]ReferencePolicy),
		sources:     make(map[ReferenceBackend]credentialSource),
		cloudChains: defaultCloudChains(),
		versionSeed: maphash.MakeSeed(),
		cache:       make(map[string]sourceMaterial),
		inflight:    make(map[string]*resolutionCall),
	}
	resolver.sources[referenceBackendEnvironment] = environmentSource{lookup: lookup}
	resolver.sources[referenceBackendFile] = fileSource{}
	resolver.handlers = map[catalogs.ProviderAuthenticationPrimitive]profileResolver{
		catalogs.ProviderAuthenticationNone:          resolver.resolveAmbientProfile,
		catalogs.ProviderAuthenticationAPIKey:        resolver.resolveAmbientProfile,
		catalogs.ProviderAuthenticationBearerToken:   resolver.resolveAmbientProfile,
		catalogs.ProviderAuthenticationGoogleDefault: resolver.resolveDefaultChainProfile,
		catalogs.ProviderAuthenticationAzureDefault:  resolver.resolveDefaultChainProfile,
		catalogs.ProviderAuthenticationAWSDefault:    resolver.resolveDefaultChainProfile,
	}
	for _, option := range options {
		if option != nil {
			option(resolver)
		}
	}
	return resolver
}

// ResolveCatalog resolves the first configured catalog-acquisition profile.
// A malformed or unavailable selected explicit source is terminal unless its
// policy permits a not-configured ambient fallback.
func (r *Resolver) ResolveCatalog(
	ctx context.Context,
	provider *catalogs.Provider,
) (sources.ProviderCredentialMaterial, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	if provider == nil || provider.Credentials == nil {
		return sources.ProviderCredentialMaterial{}, &errors.ValidationError{
			Field: "provider.credentials", Message: "catalog credential metadata is required",
		}
	}
	fields := indexCredentialFields(provider.Credentials.Fields)
	if err := r.validateReferencePolicies(provider.ID, fields); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	profiles := indexCredentialProfiles(provider.Credentials.Profiles)
	plane := provider.Credentials.CatalogAcquisition
	for _, profileID := range plane.Alternatives {
		profile, exists := profiles[profileID]
		if !exists {
			return sources.ProviderCredentialMaterial{}, &errors.ValidationError{
				Field: "provider.credentials.catalog_acquisition.alternatives",
				Value: profileID, Message: "references an unknown profile",
			}
		}
		handler, exists := r.handlers[profile.Primitive]
		if !exists {
			return sources.ProviderCredentialMaterial{}, &errors.ValidationError{
				Field: "provider.credentials.profiles.primitive",
				Value: profile.Primitive, Message: "is not supported",
			}
		}
		material, configured, err := handler(ctx, provider, profile, fields)
		if err != nil {
			return sources.ProviderCredentialMaterial{}, err
		}
		if configured {
			return material, nil
		}
	}
	if !plane.Required {
		return sources.ProviderCredentialMaterial{}, nil
	}
	return sources.ProviderCredentialMaterial{}, &errors.AuthenticationError{
		Provider: string(provider.ID),
		Method:   "catalog-declared",
		Message:  "no catalog-acquisition credential profile is configured",
	}
}

func (r *Resolver) resolveAmbientProfile(
	ctx context.Context,
	provider *catalogs.Provider,
	profile catalogs.ProviderCredentialProfile,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error) {
	builder := newMaterialBuilder()
	for _, fieldID := range profile.Fields {
		resolved, selected, err := r.resolveField(ctx, provider.ID, fields[fieldID])
		if err != nil {
			return sources.ProviderCredentialMaterial{}, false, err
		}
		if !selected && fields[fieldID].Required {
			return sources.ProviderCredentialMaterial{}, false, nil
		}
		if selected {
			builder.add(fieldID, resolved)
		}
	}
	return builder.build(r, profile), true, nil
}

func (r *Resolver) resolveDefaultChainProfile(
	ctx context.Context,
	provider *catalogs.Provider,
	profile catalogs.ProviderCredentialProfile,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error) {
	builder := newMaterialBuilder()
	missing := make(map[catalogs.ProviderCredentialFieldID]struct{})
	for _, fieldID := range profile.Fields {
		resolved, selected, err := r.resolveField(ctx, provider.ID, fields[fieldID])
		if err != nil {
			return sources.ProviderCredentialMaterial{}, false, err
		}
		if selected {
			builder.add(fieldID, resolved)
			continue
		}
		if fields[fieldID].Required {
			missing[fieldID] = struct{}{}
		}
	}
	if len(missing) > 0 {
		chain := r.cloudChains[profile.Primitive]
		if chain == nil {
			return sources.ProviderCredentialMaterial{}, false, nil
		}
		identity := r.opaqueVersion(
			"cloud-chain", string(provider.ID), string(profile.ID), string(profile.Primitive),
		)
		chainMaterial, err := r.resolveSource(ctx, identity, func(resolveCtx context.Context) (sourceMaterial, error) {
			return chain.resolve(resolveCtx, profile, fields)
		})
		if err != nil {
			if isSourceError(err, SourceErrorNotConfigured) {
				return sources.ProviderCredentialMaterial{}, false, nil
			}
			return sources.ProviderCredentialMaterial{}, false, err
		}
		for _, fieldID := range profile.Fields {
			if _, exists := builder.values[fieldID]; exists {
				continue
			}
			value, exists := chainMaterial.values[string(fieldID)]
			if !exists || value == "" {
				continue
			}
			if err := validateResolvedField(fields[fieldID], value); err != nil {
				return sources.ProviderCredentialMaterial{}, false, err
			}
			resolved := chainMaterial.copy()
			resolved.values = map[string]string{"value": value}
			builder.add(fieldID, resolvedFieldFromSource(value, resolved))
		}
	}
	for _, fieldID := range profile.Fields {
		if fields[fieldID].Required {
			if _, exists := builder.values[fieldID]; !exists {
				return sources.ProviderCredentialMaterial{}, false, nil
			}
		}
	}
	return builder.build(r, profile), true, nil
}

type resolvedField struct {
	value     string
	version   string
	expiresAt time.Time
	lease     *sources.ProviderCredentialLease
}

func (r *Resolver) resolveField(
	ctx context.Context,
	providerID catalogs.ProviderID,
	field catalogs.ProviderCredentialField,
) (resolvedField, bool, error) {
	key := CredentialFieldKey{ProviderID: providerID, FieldID: field.ID}
	if policy, exists := r.references[key]; exists {
		material, err := r.resolveReference(ctx, key, policy.Reference)
		if err == nil {
			value, selectErr := referenceValue(material, policy.Reference)
			if selectErr != nil {
				return resolvedField{}, false, selectErr
			}
			if validateErr := validateResolvedField(field, value); validateErr != nil {
				return resolvedField{}, false, validateErr
			}
			return resolvedFieldFromSource(value, material), true, nil
		}
		if !policy.FallbackAmbient || !isSourceError(err, SourceErrorNotConfigured) {
			return resolvedField{}, false, err
		}
	}
	return r.resolveAmbientField(providerID, field)
}

func (r *Resolver) resolveAmbientField(
	providerID catalogs.ProviderID,
	field catalogs.ProviderCredentialField,
) (resolvedField, bool, error) {
	candidates := append([]string(nil), field.Environment...)
	derived, err := catalogs.DerivedCredentialEnvironmentName(
		starmapCredentialProduct,
		providerID,
		field.ID,
	)
	if err != nil {
		return resolvedField{}, false, err
	}
	candidates = append(candidates, derived)
	for _, name := range candidates {
		value, exists := r.lookup(name)
		if !exists || value == "" {
			continue
		}
		if err := validateResolvedField(field, value); err != nil {
			return resolvedField{}, false, &errors.ValidationError{
				Field:   "provider.credentials.environment",
				Value:   name,
				Message: fmt.Sprintf("selected value does not match field %s", field.ID),
			}
		}
		return resolvedField{value: value, version: name + "\x00" + value}, true, nil
	}
	if field.Default != "" {
		return resolvedField{value: field.Default, version: "default\x00" + field.Default}, true, nil
	}
	return resolvedField{}, false, nil
}

func validateResolvedField(field catalogs.ProviderCredentialField, value string) error {
	if field.Pattern == "" {
		return nil
	}
	matched, err := regexp.MatchString(field.Pattern, value)
	if err != nil {
		return errors.WrapParse("regex", field.Pattern, err)
	}
	if !matched {
		return &errors.ValidationError{
			Field:   "provider.credentials.source",
			Message: fmt.Sprintf("selected value does not match field %s", field.ID),
		}
	}
	return nil
}

func resolvedFieldFromSource(value string, material sourceMaterial) resolvedField {
	var lease *sources.ProviderCredentialLease
	if material.lease != nil {
		copied := *material.lease
		lease = &copied
	}
	return resolvedField{
		value: value, version: material.version,
		expiresAt: material.expiresAt, lease: lease,
	}
}

func referenceValue(material sourceMaterial, reference Reference) (string, error) {
	if reference.field != "" {
		value, exists := material.values[reference.field]
		if !exists || value == "" {
			return "", newSourceError(SourceErrorNotConfigured, reference.backend)
		}
		return value, nil
	}
	if value, exists := material.values["value"]; exists && value != "" {
		return value, nil
	}
	if len(material.values) == 1 {
		for _, value := range material.values {
			if value != "" {
				return value, nil
			}
		}
	}
	return "", newSourceError(SourceErrorInvalid, reference.backend)
}

func (r *Resolver) resolveReference(
	ctx context.Context,
	key CredentialFieldKey,
	reference Reference,
) (sourceMaterial, error) {
	identity := r.referenceIdentity(key, reference)
	source := r.sources[reference.backend]
	if source == nil {
		return sourceMaterial{}, &errors.ValidationError{
			Field: "credential_reference.backend", Value: reference.backend,
			Message: "is not supported",
		}
	}
	return r.resolveSource(ctx, identity, func(resolveCtx context.Context) (sourceMaterial, error) {
		return source.Resolve(resolveCtx, reference)
	})
}

func (r *Resolver) resolveSource(
	ctx context.Context,
	identity string,
	resolve sourceResolve,
) (sourceMaterial, error) {
	now := time.Now()
	r.mu.Lock()
	if cached, exists := r.cache[identity]; exists && cached.fresh(now) {
		r.mu.Unlock()
		return cached.copy(), nil
	}
	if call, exists := r.inflight[identity]; exists {
		r.mu.Unlock()
		select {
		case <-call.done:
			return call.material.copy(), call.err
		case <-ctx.Done():
			return sourceMaterial{}, ctx.Err()
		}
	}
	call := &resolutionCall{done: make(chan struct{})}
	r.inflight[identity] = call
	r.mu.Unlock()

	call.material, call.err = resolve(ctx)

	r.mu.Lock()
	if call.err == nil {
		r.cache[identity] = call.material.copy()
	}
	delete(r.inflight, identity)
	close(call.done)
	r.mu.Unlock()
	return call.material.copy(), call.err
}

func (r *Resolver) validateReferencePolicies(
	providerID catalogs.ProviderID,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) error {
	for key := range r.references {
		if key.ProviderID != providerID {
			continue
		}
		if _, exists := fields[key.FieldID]; !exists {
			return &errors.ValidationError{
				Field: "credential_sources.field", Value: key.FieldID,
				Message: "does not reference a catalog-declared provider field",
			}
		}
	}
	return nil
}

func (r *Resolver) referenceIdentity(key CredentialFieldKey, reference Reference) string {
	return r.opaqueVersion(
		"reference",
		string(key.ProviderID),
		string(key.FieldID),
		string(reference.backend),
		reference.resource,
		reference.field,
		reference.version,
	)
}

func (r *Resolver) opaqueVersion(parts ...string) string {
	var hash maphash.Hash
	hash.SetSeed(r.versionSeed)
	for _, part := range parts {
		_, _ = hash.WriteString(part)
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("v1-%016x", hash.Sum64())
}

type materialBuilder struct {
	values   map[catalogs.ProviderCredentialFieldID]string
	versions map[catalogs.ProviderCredentialFieldID]string
	expires  time.Time
	lease    *sources.ProviderCredentialLease
}

func newMaterialBuilder() *materialBuilder {
	return &materialBuilder{
		values:   make(map[catalogs.ProviderCredentialFieldID]string),
		versions: make(map[catalogs.ProviderCredentialFieldID]string),
	}
}

func (b *materialBuilder) add(fieldID catalogs.ProviderCredentialFieldID, resolved resolvedField) {
	b.values[fieldID] = resolved.value
	b.versions[fieldID] = resolved.version
	if !resolved.expiresAt.IsZero() && (b.expires.IsZero() || resolved.expiresAt.Before(b.expires)) {
		b.expires = resolved.expiresAt
	}
	if resolved.lease == nil {
		return
	}
	if b.lease == nil {
		lease := *resolved.lease
		b.lease = &lease
		return
	}
	b.lease.Renewable = b.lease.Renewable || resolved.lease.Renewable
	if b.lease.RefreshAfter.IsZero() ||
		(!resolved.lease.RefreshAfter.IsZero() && resolved.lease.RefreshAfter.Before(b.lease.RefreshAfter)) {
		b.lease.RefreshAfter = resolved.lease.RefreshAfter
	}
}

func (b *materialBuilder) build(
	resolver *Resolver,
	profile catalogs.ProviderCredentialProfile,
) sources.ProviderCredentialMaterial {
	fieldIDs := make([]string, 0, len(b.values))
	for fieldID := range b.values {
		fieldIDs = append(fieldIDs, string(fieldID))
	}
	sort.Strings(fieldIDs)
	versionParts := make([]string, 0, 3+3*len(fieldIDs))
	versionParts = append(versionParts, "material", string(profile.ID), string(profile.Primitive))
	for _, fieldValue := range fieldIDs {
		fieldID := catalogs.ProviderCredentialFieldID(fieldValue)
		versionParts = append(versionParts, fieldValue, b.versions[fieldID], b.values[fieldID])
	}
	return sources.NewProviderCredentialMaterial(
		profile,
		b.values,
		sources.ProviderCredentialMetadata{
			Version:   resolver.opaqueVersion(versionParts...),
			ExpiresAt: b.expires,
			Lease:     b.lease,
		},
	)
}

func indexCredentialFields(
	fields []catalogs.ProviderCredentialField,
) map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField {
	indexed := make(map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField, len(fields))
	for _, field := range fields {
		indexed[field.ID] = field
	}
	return indexed
}

func indexCredentialProfiles(
	profiles []catalogs.ProviderCredentialProfile,
) map[catalogs.ProviderCredentialProfileID]catalogs.ProviderCredentialProfile {
	indexed := make(map[catalogs.ProviderCredentialProfileID]catalogs.ProviderCredentialProfile, len(profiles))
	for _, profile := range profiles {
		indexed[profile.ID] = profile
	}
	return indexed
}
