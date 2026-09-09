package sources

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderAcquisitionBindingSchemaVersion identifies the supported binding format.
const ProviderAcquisitionBindingSchemaVersion = 2

// MaxProviderBindingFieldBytes bounds each declared binding identifier or selector.
const MaxProviderBindingFieldBytes = 4096

// ProviderBindingCredentialRole identifies the purpose of a binding's credentials.
type ProviderBindingCredentialRole string

// ProviderBindingCatalogAcquisition identifies catalog-acquisition credentials.
const ProviderBindingCatalogAcquisition ProviderBindingCredentialRole = "catalog_acquisition"

// ProviderMembershipAuthority declares which serving membership a binding may replace.
// Completeness remains a separate requirement for each observation.
type ProviderMembershipAuthority string

const (
	// ProviderMembershipEvidenceOnly grants no membership replacement authority.
	ProviderMembershipEvidenceOnly ProviderMembershipAuthority = ""
	// ProviderMembershipScope permits a binding to replace membership only within its declared scope.
	ProviderMembershipScope ProviderMembershipAuthority = "scope"
	// ProviderMembershipProvider permits a binding to replace the provider's public membership.
	ProviderMembershipProvider ProviderMembershipAuthority = "provider"
)

// ProviderAcquisitionBinding declares one provider scope under a deployment-owned revision.
// It contains no credential material and does not prove upstream account ownership or completeness.
type ProviderAcquisitionBinding struct {
	// SchemaVersion selects the binding wire contract.
	SchemaVersion int `json:"schema_version" yaml:"schema_version"`
	// ID is the stable deployment-owned binding identity.
	ID string `json:"id" yaml:"id"`
	// Revision changes when the declared scope or credential role changes.
	Revision string `json:"revision" yaml:"revision"`
	// ProviderID names the single canonical provider in the observation.
	ProviderID catalogs.ProviderID `json:"provider_id" yaml:"provider_id"`
	// Public declares a scope without account or project selectors.
	Public bool `json:"public,omitzero" yaml:"public,omitempty"`
	// AccountID is the declared provider-account identifier.
	AccountID string `json:"account_id,omitempty" yaml:"account_id,omitempty"`
	// ProjectID is the declared provider-project identifier.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// Region names the declared region, including an explicit global scope.
	Region string `json:"region" yaml:"region"`
	// APISurface names the provider operation that produced the observed records.
	APISurface string `json:"api_surface" yaml:"api_surface"`
	// MembershipAuthority declares replacement permission independently of reply completeness.
	MembershipAuthority ProviderMembershipAuthority `json:"membership_authority,omitempty" yaml:"membership_authority,omitempty"`
	// CredentialRole identifies catalog acquisition, never inference.
	CredentialRole ProviderBindingCredentialRole `json:"credential_role" yaml:"credential_role"`
	// CredentialProfileID names the declared authentication profile, without credential material.
	CredentialProfileID catalogs.ProviderCredentialProfileID `json:"credential_profile_id" yaml:"credential_profile_id"`
}

// UnmarshalJSON rejects unknown fields and unsupported binding schemas.
// Failed decoding leaves the receiver unchanged.
func (b *ProviderAcquisitionBinding) UnmarshalJSON(data []byte) error {
	type bindingJSON ProviderAcquisitionBinding
	var decoded bindingJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return providerBindingError("format", "must be a binding object with known fields")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return providerBindingError("format", "must contain exactly one binding object")
	}
	binding := ProviderAcquisitionBinding(decoded)
	if err := binding.Validate(); err != nil {
		return err
	}
	*b = binding
	return nil
}

// Validate checks the binding's format and requires an explicit public or account/project scope.
// Errors identify fields without exposing their values.
func (b ProviderAcquisitionBinding) Validate() error {
	if b.SchemaVersion != 1 && b.SchemaVersion != ProviderAcquisitionBindingSchemaVersion {
		return providerBindingError("schema_version", "is not supported")
	}
	required := []struct{ name, value string }{
		{"id", b.ID}, {"revision", b.Revision}, {"provider_id", string(b.ProviderID)},
		{"region", b.Region}, {"api_surface", b.APISurface}, {"credential_profile_id", string(b.CredentialProfileID)},
	}
	for _, field := range required {
		if err := validateProviderBindingField(field.name, field.value, true); err != nil {
			return err
		}
	}
	for _, field := range []struct{ name, value string }{{"account_id", b.AccountID}, {"project_id", b.ProjectID}} {
		if err := validateProviderBindingField(field.name, field.value, false); err != nil {
			return err
		}
	}
	if b.CredentialRole != ProviderBindingCatalogAcquisition {
		return providerBindingError("credential_role", "must be catalog_acquisition")
	}
	if b.Public && (b.AccountID != "" || b.ProjectID != "") {
		return providerBindingError("public", "cannot also declare an account or project")
	}
	if !b.Public && b.AccountID == "" && b.ProjectID == "" {
		return providerBindingError("scope", "must declare public scope or an account or project")
	}
	if b.SchemaVersion == 1 && b.MembershipAuthority != ProviderMembershipEvidenceOnly {
		return providerBindingError("membership_authority", "requires binding schema version 2")
	}
	switch b.MembershipAuthority {
	case ProviderMembershipEvidenceOnly, ProviderMembershipScope:
	case ProviderMembershipProvider:
		if !b.Public {
			return providerBindingError("membership_authority", "provider replacement requires public scope")
		}
	default:
		return providerBindingError("membership_authority", "is not supported")
	}
	return nil
}

func validateProviderBindingField(name, value string, required bool) error {
	if value == "" && !required {
		return nil
	}
	if value == "" || len(value) > MaxProviderBindingFieldBytes || strings.TrimSpace(value) != value || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return providerBindingError(name, "must be a bounded UTF-8 identifier without control characters or surrounding whitespace")
	}
	return nil
}

func providerBindingError(field, message string) error {
	return &errors.ValidationError{Field: "provider_binding." + field, Message: message}
}

func cloneProviderBinding(binding *ProviderAcquisitionBinding) *ProviderAcquisitionBinding {
	if binding == nil {
		return nil
	}
	clone := *binding
	return &clone
}

func (b ProviderAcquisitionBinding) identityFields() []string {
	fields := []string{strconv.Itoa(b.SchemaVersion), b.ID, b.Revision, string(b.ProviderID), strconv.FormatBool(b.Public), b.AccountID, b.ProjectID, b.Region, b.APISurface, string(b.CredentialRole), string(b.CredentialProfileID)}
	if b.SchemaVersion >= 2 {
		fields = append(fields, string(b.MembershipAuthority))
	}
	return fields
}

func (o Observation) validateProviderBinding() error {
	if o.ProviderBinding == nil {
		return nil
	}
	if err := o.ProviderBinding.Validate(); err != nil {
		return err
	}
	if o.SourceID != ProvidersID {
		return providerBindingError("source", "requires a provider source observation")
	}
	if o.Catalog.Providers().Len() != 1 {
		return providerBindingError("provider_id", "requires exactly one observed provider")
	}
	if _, found := o.Catalog.Providers().Get(o.ProviderBinding.ProviderID); !found {
		return providerBindingError("provider_id", "does not match the observed provider")
	}
	return nil
}
