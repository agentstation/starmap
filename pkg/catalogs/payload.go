package catalogs

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// CatalogPayload is the canonical provider-oriented JSON representation.
// Provider models are the only persisted model records. Definitions,
// offerings, and author membership are derived immutable read views.
type CatalogPayload struct {
	SchemaVersion  uint64             `json:"schema_version"`
	Providers      []Provider         `json:"providers"`
	Authors        []Author           `json:"authors"`
	Endpoints      []Endpoint         `json:"endpoints"`
	ProviderModels map[string][]Model `json:"provider_models"`
	Provenance     provenance.Map     `json:"provenance"`
}

// EncodeCatalogPayload deterministically encodes a readable catalog.
func EncodeCatalogPayload(reader Reader) ([]byte, error) {
	if reader == nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: "is required"}
	}
	payload := CatalogPayload{
		SchemaVersion:  CurrentCatalogSchemaVersion,
		Providers:      reader.Providers().List(),
		Authors:        reader.Authors().List(),
		Endpoints:      reader.Endpoints().List(),
		ProviderModels: make(map[string][]Model),
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
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: fmt.Sprintf("cannot encode payload: %v", err)}
	}
	return data, nil
}
