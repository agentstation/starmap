package catalogs

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// CatalogPayload is catalog schema v1's canonical JSON representation.
// Provider and author models are separate maps because their legacy structs do
// not serialize runtime model indexes.
type CatalogPayload struct {
	SchemaVersion  uint64             `json:"schema_version"`
	Providers      []Provider         `json:"providers"`
	Authors        []Author           `json:"authors"`
	Endpoints      []Endpoint         `json:"endpoints"`
	ProviderModels map[string][]Model `json:"provider_models"`
	AuthorModels   map[string][]Model `json:"author_models"`
	Provenance     provenance.Map     `json:"provenance"`
}

// EncodeCatalogPayload deterministically encodes a readable catalog as schema v1.
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
		AuthorModels:   make(map[string][]Model),
		Provenance:     reader.Provenance().Map(),
	}
	for _, provider := range payload.Providers {
		models, err := reader.ProviderModels(provider.ID)
		if err != nil {
			return nil, err
		}
		payload.ProviderModels[string(provider.ID)] = models.List()
	}
	for _, author := range payload.Authors {
		models := make([]Model, 0, len(author.Models))
		for _, model := range author.Models {
			if model != nil {
				models = append(models, DeepCopyModel(*model))
			}
		}
		slices.SortFunc(models, func(left, right Model) int {
			if left.ID < right.ID {
				return -1
			}
			if left.ID > right.ID {
				return 1
			}
			return 0
		})
		payload.AuthorModels[string(author.ID)] = models
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: fmt.Sprintf("cannot encode payload: %v", err)}
	}
	return data, nil
}
