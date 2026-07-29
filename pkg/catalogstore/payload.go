package catalogstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

// EncodeCatalogPayload deterministically encodes a readable catalog.
func EncodeCatalogPayload(reader catalogs.Reader) ([]byte, error) {
	return catalogs.EncodeCatalogPayload(reader)
}

type payloadDecodeReport struct {
	ProviderModels sourcepayload.RecordReport
}

type payloadEnvelope struct {
	SchemaVersion  uint64                     `json:"schema_version"`
	Providers      []catalogs.Provider        `json:"providers"`
	Authors        []catalogs.Author          `json:"authors"`
	Endpoints      []catalogs.Endpoint        `json:"endpoints"`
	ProviderModels map[string]json.RawMessage `json:"provider_models"`
	Provenance     provenance.Map             `json:"provenance"`
}

func (r payloadDecodeReport) err() error {
	return r.ProviderModels.Err("catalog payload models")
}

// DecodeCatalogPayload decodes the current catalog payload. A non-nil catalog together
// with *sourcepayload.QuarantineError is a partial diagnostic result and must
// not be activated as the manifest-bound generation.
func DecodeCatalogPayload(data []byte) (*catalogs.Catalog, error) {
	catalog, report, err := decodeCatalogPayload(data)
	if err != nil {
		return nil, err
	}
	return catalog, report.err()
}

func decodeCatalogPayload(data []byte) (*catalogs.Catalog, payloadDecodeReport, error) {
	payload, err := decodePayloadEnvelope(data)
	if err != nil {
		return nil, payloadDecodeReport{}, err
	}
	return buildDecodedCatalog(payload)
}

func decodePayloadEnvelope(data []byte) (payloadEnvelope, error) {
	if err := sourcepayload.ValidateJSON(data); err != nil {
		return payloadEnvelope{}, err
	}
	var required map[string]json.RawMessage
	if err := json.Unmarshal(data, &required); err != nil {
		return payloadEnvelope{}, &errors.ParseError{
			Format: "json", File: "catalog payload", Message: err.Error(), Err: err,
		}
	}
	for _, field := range []string{
		"schema_version", "providers", "authors", "endpoints",
		"provider_models", "provenance",
	} {
		if _, found := required[field]; !found {
			return payloadEnvelope{}, &errors.ValidationError{Field: field, Message: "is required"}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload payloadEnvelope
	if err := decoder.Decode(&payload); err != nil {
		return payloadEnvelope{}, &errors.ParseError{
			Format: "json", File: "catalog payload", Message: err.Error(), Err: err,
		}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return payloadEnvelope{}, &errors.ParseError{
			Format: "json", File: "catalog payload", Message: "invalid trailing JSON", Err: err,
		}
	}
	if payload.SchemaVersion != catalogs.CurrentCatalogSchemaVersion {
		return payloadEnvelope{}, &errors.ValidationError{
			Field:   "schema_version",
			Value:   payload.SchemaVersion,
			Message: fmt.Sprintf("must be %d", catalogs.CurrentCatalogSchemaVersion),
		}
	}
	for _, field := range []struct {
		name   string
		isNull bool
	}{
		{name: "providers", isNull: payload.Providers == nil},
		{name: "authors", isNull: payload.Authors == nil},
		{name: "endpoints", isNull: payload.Endpoints == nil},
		{name: "provider_models", isNull: payload.ProviderModels == nil},
		{name: "provenance", isNull: payload.Provenance == nil},
	} {
		if field.isNull {
			return payloadEnvelope{}, &errors.ValidationError{
				Field: field.name, Message: "must not be null",
			}
		}
	}
	if len(payload.Providers) > constants.MaxProviders || len(payload.ProviderModels) > constants.MaxProviders {
		return payloadEnvelope{}, &errors.ValidationError{
			Field: "providers", Value: len(payload.Providers), Message: "exceeds maximum provider count",
		}
	}
	if len(payload.Authors) > constants.MaxCatalogModels || len(payload.Endpoints) > constants.MaxCatalogModels {
		return payloadEnvelope{}, &errors.ValidationError{
			Field: "catalog", Message: "author or endpoint count exceeds maximum",
		}
	}
	return payload, nil
}

func buildDecodedCatalog(payload payloadEnvelope) (*catalogs.Catalog, payloadDecodeReport, error) {
	builder := catalogs.NewEmpty()
	providerIDs := make(map[string]struct{}, len(payload.Providers))
	for _, provider := range payload.Providers {
		if _, exists := providerIDs[string(provider.ID)]; exists {
			return nil, payloadDecodeReport{}, &errors.ValidationError{
				Field: "providers.id", Value: provider.ID, Message: "must be unique",
			}
		}
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			return nil, payloadDecodeReport{}, errors.WrapResource("decode", "provider", string(provider.ID), err)
		}
		providerIDs[string(provider.ID)] = struct{}{}
	}
	providerKeys := sortedRawKeys(payload.ProviderModels)
	report := payloadDecodeReport{}
	for providerID := range providerIDs {
		if _, found := payload.ProviderModels[providerID]; !found {
			return nil, payloadDecodeReport{}, &errors.ValidationError{
				Field: "provider_models", Value: providerID, Message: "is required for every provider",
			}
		}
	}
	for _, providerID := range providerKeys {
		if _, found := providerIDs[providerID]; !found {
			return nil, payloadDecodeReport{}, &errors.ValidationError{
				Field: "provider_models", Value: providerID, Message: "references an unknown provider",
			}
		}
		remaining := remainingRecordBudget(report.ProviderModels)
		models, recordReport, err := sourcepayload.DecodeJSONArray[catalogs.Model](
			payload.ProviderModels[providerID],
			"provider_models["+providerID+"]",
			remaining,
		)
		if err != nil {
			return nil, payloadDecodeReport{}, err
		}
		mergeRecordReport(&report.ProviderModels, recordReport)
		for _, model := range models {
			if err := builder.SetProviderModel(catalogs.ProviderID(providerID), model); err != nil {
				report.ProviderModels.Accepted--
				report.ProviderModels.Rejected++
				report.ProviderModels.Issues = append(report.ProviderModels.Issues, sourcepayload.RecordIssue{
					Subject: "provider_models[" + providerID + "]/" + model.ID,
					Err:     errors.WrapResource("decode", "provider model", providerID+"/"+model.ID, err),
				})
			}
		}
	}
	authorIDs := make(map[string]struct{}, len(payload.Authors))
	for _, author := range payload.Authors {
		if _, exists := authorIDs[string(author.ID)]; exists {
			return nil, payloadDecodeReport{}, &errors.ValidationError{
				Field: "authors.id", Value: author.ID, Message: "must be unique",
			}
		}
		authorIDs[string(author.ID)] = struct{}{}
		if err := builder.SetAuthor(author); err != nil {
			return nil, payloadDecodeReport{}, errors.WrapResource("decode", "author", string(author.ID), err)
		}
	}
	endpointIDs := make(map[string]struct{}, len(payload.Endpoints))
	for _, endpoint := range payload.Endpoints {
		if _, exists := endpointIDs[endpoint.ID]; exists {
			return nil, payloadDecodeReport{}, &errors.ValidationError{
				Field: "endpoints.id", Value: endpoint.ID, Message: "must be unique",
			}
		}
		endpointIDs[endpoint.ID] = struct{}{}
		if err := builder.SetEndpoint(endpoint); err != nil {
			return nil, payloadDecodeReport{}, errors.WrapResource("decode", "endpoint", endpoint.ID, err)
		}
	}
	builder.SetProvenance(payload.Provenance)
	catalog, err := builder.Build()
	return catalog, report, err
}

func sortedRawKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mergeRecordReport(target *sourcepayload.RecordReport, addition sourcepayload.RecordReport) {
	target.Accepted += addition.Accepted
	target.Rejected += addition.Rejected
	target.Truncated = target.Truncated || addition.Truncated
	target.Issues = append(target.Issues, addition.Issues...)
}

func remainingRecordBudget(report sourcepayload.RecordReport) int {
	used := report.Accepted + report.Rejected
	if used >= constants.MaxCatalogModels {
		return 0
	}
	return constants.MaxCatalogModels - used
}
