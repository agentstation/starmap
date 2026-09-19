package catalog

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// acquisitionCredentialProduct is the product prefix of a derived catalog
// acquisition name. It is the gateway prefix, so a deployment names an
// acquisition credential exactly as it names every other gateway setting.
const acquisitionCredentialProduct = "STARPORT"

// DeploymentLookup reads one deployment environment name. It is the only
// credential source catalog acquisition reads.
type DeploymentLookup func(name string) (string, bool)

// AcquisitionResolver resolves one catalog-acquisition credential from the
// deployment lookup alone.
//
// Catalog acquisition is deployment work, never account work. An account
// credential pays a provider for that account's inference, and a shared
// credential the operator grants pays for a group of accounts. Neither one
// belongs to the process that reads a provider catalog, so this resolver holds
// exactly one field: the deployment lookup. It can reach no keyring, no
// account store, and no BYOK record, because it does not hold one.
type AcquisitionResolver struct {
	lookup DeploymentLookup
}

// NewAcquisitionResolver returns the deployment acquisition resolver. A nil
// lookup resolves nothing, so every provider becomes ineligible instead of
// silently reading another credential plane.
func NewAcquisitionResolver(lookup DeploymentLookup) *AcquisitionResolver {
	return &AcquisitionResolver{lookup: lookup}
}

// ResolveCatalog selects the first catalog-acquisition profile whose required
// fields the deployment supplies. It returns a typed error when the deployment
// supplies none, and the provider then stays out of the observation run.
func (r *AcquisitionResolver) ResolveCatalog(
	_ context.Context,
	provider *catalogs.Provider,
) (sources.ProviderCredentialMaterial, error) {
	if r == nil || r.lookup == nil {
		return sources.ProviderCredentialMaterial{}, &starmaperrors.ConfigError{
			Component: "catalog acquisition",
			Message:   "the deployment credential lookup is required",
		}
	}
	if provider == nil || provider.Credentials == nil {
		return sources.ProviderCredentialMaterial{}, &starmaperrors.ValidationError{
			Field:   "provider.credentials",
			Message: "is required",
		}
	}
	plane := provider.Credentials.CatalogAcquisition
	fields := indexCredentialFields(provider.Credentials.Fields)
	for _, profileID := range plane.Alternatives {
		profile, found := findCredentialProfile(provider.Credentials.Profiles, profileID)
		if !found {
			continue
		}
		values, complete := r.resolveProfile(provider.ID, profile, fields)
		if !complete {
			continue
		}
		return sources.NewProviderCredentialMaterial(
			profile,
			values,
			sources.ProviderCredentialMetadata{Version: "deployment"},
		), nil
	}
	if !plane.Required {
		return sources.NewProviderCredentialMaterial(
			catalogs.ProviderCredentialProfile{},
			nil,
			sources.ProviderCredentialMetadata{Version: "deployment"},
		), nil
	}
	return sources.ProviderCredentialMaterial{}, &starmaperrors.NotFoundError{
		Resource: "catalog acquisition credential",
		ID:       string(provider.ID),
	}
}

// resolveProfile reads every field of one profile. It reports whether the
// deployment supplied each required field.
func (r *AcquisitionResolver) resolveProfile(
	providerID catalogs.ProviderID,
	profile catalogs.ProviderCredentialProfile,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (map[catalogs.ProviderCredentialFieldID]string, bool) {
	values := make(map[catalogs.ProviderCredentialFieldID]string, len(profile.Fields))
	for _, fieldID := range profile.Fields {
		field := fields[fieldID]
		value, found := r.readField(providerID, fieldID, field)
		switch {
		case found:
			values[fieldID] = value
		case field.Default != "":
			values[fieldID] = field.Default
		case field.Required:
			return nil, false
		}
	}
	return values, true
}

// readField reads one field. The derived gateway name wins, and the
// conventional ambient names follow it in catalog order.
func (r *AcquisitionResolver) readField(
	providerID catalogs.ProviderID,
	fieldID catalogs.ProviderCredentialFieldID,
	field catalogs.ProviderCredentialField,
) (string, bool) {
	derived, err := catalogs.DerivedCredentialEnvironmentName(
		acquisitionCredentialProduct, providerID, fieldID,
	)
	if err == nil {
		if value, found := r.read(derived); found {
			return value, true
		}
	}
	for _, name := range field.Environment {
		if value, found := r.read(name); found {
			return value, true
		}
	}
	return "", false
}

// read returns one non-empty deployment value.
func (r *AcquisitionResolver) read(name string) (string, bool) {
	if strings.TrimSpace(name) == "" {
		return "", false
	}
	value, found := r.lookup(name)
	if !found || strings.TrimSpace(value) == "" {
		return "", false
	}
	return value, true
}

func indexCredentialFields(
	fields []catalogs.ProviderCredentialField,
) map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField {
	index := make(map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField, len(fields))
	for _, field := range fields {
		index[field.ID] = field
	}
	return index
}

func findCredentialProfile(
	profiles []catalogs.ProviderCredentialProfile,
	id catalogs.ProviderCredentialProfileID,
) (catalogs.ProviderCredentialProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return catalogs.ProviderCredentialProfile{}, false
}

var _ sources.ProviderCredentialResolver = (*AcquisitionResolver)(nil)
