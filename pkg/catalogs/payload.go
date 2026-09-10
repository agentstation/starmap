package catalogs

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

// CatalogPayload is the canonical construction-record JSON representation.
// Author models own provider-independent facts. Provider models own serving
// facts and link to author models through Model.ModelRef.
type CatalogPayload struct {
	SchemaVersion    uint64                    `json:"schema_version"`
	Providers        []Provider                `json:"providers"`
	Authors          []Author                  `json:"authors"`
	ProviderModels   map[string][]Model        `json:"provider_models"`
	AuthorModels     map[string][]Model        `json:"author_models"`
	Provenance       provenance.Map            `json:"provenance"`
	MembershipScopes []ProviderMembershipScope `json:"membership_scopes,omitempty"`
	RemovalPolicies  []CatalogRemovalPolicy    `json:"removal_policies,omitempty"`
	CanonicalAliases []CanonicalAlias          `json:"canonical_aliases,omitempty"`
}

// EncodeCatalogPayload deterministically encodes a readable catalog.
func EncodeCatalogPayload(reader Reader) ([]byte, error) {
	payload, err := catalogPayload(reader)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: fmt.Sprintf("cannot encode payload: %v", err)}
	}
	if err := sourcepayload.ValidateJSONWithMaxBytes(data, resourcepolicy.MaxPayloadBytes); err != nil {
		return nil, err
	}
	return data, nil
}

// CatalogSemanticChecksum identifies catalog facts and effective scope state.
// It excludes field provenance. Scope evidence renewals remain part of the identity.
// EncodeCatalogPayload binds all evidence for storage, transport, and audit.
func CatalogSemanticChecksum(reader Reader) (string, error) {
	payload, err := catalogPayload(reader)
	if err != nil {
		return "", err
	}
	payload.Provenance = nil
	data, err := json.Marshal(payload)
	if err != nil {
		return "", &errors.ValidationError{
			Field:   "catalog",
			Message: fmt.Sprintf("cannot encode semantic catalog: %v", err),
		}
	}
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest), nil
}

func catalogPayload(reader Reader) (CatalogPayload, error) {
	if reader == nil {
		return CatalogPayload{}, &errors.ValidationError{Field: "catalog", Message: "is required"}
	}
	payload := CatalogPayload{
		SchemaVersion:  CurrentCatalogSchemaVersion,
		Providers:      reader.Providers().List(),
		Authors:        reader.Authors().List(),
		ProviderModels: make(map[string][]Model),
		AuthorModels:   make(map[string][]Model),
		Provenance:     reader.Provenance().Map(),
	}

	payload.MembershipScopes = reader.MembershipScopes()
	payload.RemovalPolicies = reader.RemovalPolicies()
	payload.CanonicalAliases = reader.CanonicalAliasRecords()
	payload.SchemaVersion = CatalogPayloadSchemaVersion(reader)
	for _, provider := range payload.Providers {
		modelIDs := make([]string, 0, len(provider.Models))
		for modelID := range provider.Models {
			modelIDs = append(modelIDs, modelID)
		}
		slices.Sort(modelIDs)
		models := make([]Model, 0, len(modelIDs))
		for _, modelID := range modelIDs {
			if model := provider.Models[modelID]; model != nil {
				models = append(models, DeepCopyModel(*model))
			}
		}
		payload.ProviderModels[string(provider.ID)] = models
	}
	for _, author := range payload.Authors {
		payload.AuthorModels[string(author.ID)] = []Model{}
	}
	for _, record := range reader.AuthoredModels() {
		authorID := string(record.AuthorID)
		payload.AuthorModels[authorID] = append(
			payload.AuthorModels[authorID],
			DeepCopyModel(record.Model),
		)
	}
	return payload, nil
}

const legacyCatalogSchemaVersion uint64 = 6
const membershipCatalogSchemaVersion uint64 = 7

// SupportsCatalogSchema reports the formats this release can read and enforce.
// Version 7 adds effective scopes. Version 8 adds operator removal policies. Version 9 adds canonical rename history.
func SupportsCatalogSchema(version uint64) bool {
	return version == legacyCatalogSchemaVersion || version == membershipCatalogSchemaVersion || version == CatalogRemovalSchemaVersion || version == CurrentCatalogSchemaVersion
}

// CatalogPayloadSchemaVersion reports the schema used when encoding this reader.
// Decoded legacy evidence keeps its original schema while it has no scope records.
func CatalogPayloadSchemaVersion(reader Reader) uint64 {
	if original, ok := reader.(*Catalog); ok && len(original.CanonicalAliasRecords()) == 0 {
		if original.payloadSchemaVersion == CatalogRemovalSchemaVersion {
			return CatalogRemovalSchemaVersion
		}
		if len(original.RemovalPolicies()) != 0 {
			return CurrentCatalogSchemaVersion
		}
		if original.payloadSchemaVersion == legacyCatalogSchemaVersion && len(original.MembershipScopes()) == 0 {
			return legacyCatalogSchemaVersion
		}
		if original.payloadSchemaVersion == membershipCatalogSchemaVersion {
			return membershipCatalogSchemaVersion
		}
	}
	return CurrentCatalogSchemaVersion
}
