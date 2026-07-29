package catalogs

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// CatalogPayload is the canonical construction-record JSON representation.
// Author models own provider-independent facts; provider models own serving
// facts and link to author models through Model.ModelRef.
type CatalogPayload struct {
	SchemaVersion  uint64             `json:"schema_version"`
	Providers      []Provider         `json:"providers"`
	Authors        []Author           `json:"authors"`
	ProviderModels map[string][]Model `json:"provider_models"`
	AuthorModels   map[string][]Model `json:"author_models"`
	Provenance     provenance.Map     `json:"provenance"`
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
	return data, nil
}

// CatalogSemanticChecksum returns the stable SHA-256 identity of catalog facts.
// It excludes provenance and observation evidence; EncodeCatalogPayload remains
// the exact integrity representation for storage, transport, and audit.
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
