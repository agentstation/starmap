package catalogs

import (
	"encoding/json"
	"fmt"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
)

// CatalogPayload is catalog schema v2's canonical JSON representation.
// Definitions and offerings are explicit so enterprise service facts survive
// publication without reconstruction from ingestion-only model records.
type CatalogPayload struct {
	SchemaVersion uint64             `json:"schema_version"`
	Providers     []Provider         `json:"providers"`
	Authors       []Author           `json:"authors"`
	Endpoints     []Endpoint         `json:"endpoints"`
	Definitions   []ModelDefinition  `json:"definitions"`
	Offerings     []ProviderOffering `json:"offerings"`
	Provenance    provenance.Map     `json:"provenance"`
}

// EncodeCatalogPayload deterministically encodes a readable catalog as schema v2.
func EncodeCatalogPayload(reader Reader) ([]byte, error) {
	if reader == nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: "is required"}
	}
	payload := CatalogPayload{
		SchemaVersion: CurrentCatalogSchemaVersion,
		Providers:     reader.Providers().List(),
		Authors:       reader.Authors().List(),
		Endpoints:     reader.Endpoints().List(),
		Definitions:   reader.Definitions(),
		Offerings:     reader.Offerings(),
		Provenance:    reader.Provenance().Map(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, &errors.ValidationError{Field: "catalog", Message: fmt.Sprintf("cannot encode payload: %v", err)}
	}
	return data, nil
}
