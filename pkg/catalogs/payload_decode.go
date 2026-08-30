package catalogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/provenance"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

type payloadDecodeReport struct {
	ProviderModels sourcepayload.RecordReport
	AuthorModels   sourcepayload.RecordReport
}

type payloadEnvelope struct {
	SchemaVersion  uint64                     `json:"schema_version"`
	Providers      []Provider                 `json:"providers"`
	Authors        []Author                   `json:"authors"`
	ProviderModels map[string]json.RawMessage `json:"provider_models"`
	AuthorModels   map[string]json.RawMessage `json:"author_models"`
	Provenance     provenance.Map             `json:"provenance"`
}

func (r payloadDecodeReport) err() error {
	combined := r.ProviderModels
	mergeRecordReport(&combined, r.AuthorModels)
	return combined.Err("catalog payload models")
}

// DecodeCatalogPayload decodes the current catalog payload. A non-nil catalog
// with *sourcepayload.QuarantineError is only a partial diagnostic result. Callers
// must not activate it as the manifest-bound generation.
func DecodeCatalogPayload(data []byte) (*Catalog, error) {
	catalog, report, err := decodeCatalogPayload(data, (*Builder).Build)
	if err != nil {
		return nil, err
	}
	return catalog, report.err()
}

// DecodeSourceObservationPayload decodes a source candidate without requiring
// resolved canonical authorship for every provider record. The returned
// catalog is suitable only for reconciliation. Durable generation activation
// must use DecodeCatalogPayload.
func DecodeSourceObservationPayload(data []byte) (*Catalog, error) {
	catalog, report, err := decodeCatalogPayload(data, func(builder *Builder) (*Catalog, error) {
		return NewObservationCatalog(builder)
	})
	if err != nil {
		return nil, err
	}
	return catalog, report.err()
}

type catalogBuilder func(*Builder) (*Catalog, error)

func decodeCatalogPayload(data []byte, build catalogBuilder) (*Catalog, payloadDecodeReport, error) {
	payload, err := decodePayloadEnvelope(data)
	if err != nil {
		return nil, payloadDecodeReport{}, err
	}
	return buildDecodedCatalog(payload, build)
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
		"schema_version", "providers", "authors",
		"provider_models", "author_models", "provenance",
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
	if payload.SchemaVersion != CurrentCatalogSchemaVersion {
		return payloadEnvelope{}, &errors.ValidationError{
			Field:   "schema_version",
			Value:   payload.SchemaVersion,
			Message: fmt.Sprintf("must be %d", CurrentCatalogSchemaVersion),
		}
	}
	for _, field := range []struct {
		name   string
		isNull bool
	}{
		{name: "providers", isNull: payload.Providers == nil},
		{name: "authors", isNull: payload.Authors == nil},
		{name: "provider_models", isNull: payload.ProviderModels == nil},
		{name: "author_models", isNull: payload.AuthorModels == nil},
		{name: "provenance", isNull: payload.Provenance == nil},
	} {
		if field.isNull {
			return payloadEnvelope{}, &errors.ValidationError{
				Field: field.name, Message: "must not be null",
			}
		}
	}
	if len(payload.Providers) > resourcepolicy.MaxProviders || len(payload.ProviderModels) > resourcepolicy.MaxProviders {
		return payloadEnvelope{}, &errors.ValidationError{
			Field: "providers", Value: len(payload.Providers), Message: "exceeds maximum provider count",
		}
	}
	if len(payload.Authors) > resourcepolicy.MaxModels {
		return payloadEnvelope{}, &errors.ValidationError{
			Field: "catalog", Message: "author count exceeds maximum",
		}
	}
	return payload, nil
}

func buildDecodedCatalog(payload payloadEnvelope, build catalogBuilder) (*Catalog, payloadDecodeReport, error) {
	builder := NewEmpty()
	providerReport, err := decodePayloadProviders(builder, payload)
	if err != nil {
		return nil, payloadDecodeReport{}, err
	}
	authorReport, err := decodePayloadAuthors(builder, payload, providerReport)
	if err != nil {
		return nil, payloadDecodeReport{}, err
	}
	builder.SetProvenance(payload.Provenance)
	catalog, err := build(builder)
	return catalog, payloadDecodeReport{
		ProviderModels: providerReport,
		AuthorModels:   authorReport,
	}, err
}

func decodePayloadProviders(
	builder *Builder,
	payload payloadEnvelope,
) (sourcepayload.RecordReport, error) {
	providerIDs := make(map[string]struct{}, len(payload.Providers))
	for _, provider := range payload.Providers {
		if _, exists := providerIDs[string(provider.ID)]; exists {
			return sourcepayload.RecordReport{}, &errors.ValidationError{
				Field: "providers.id", Value: provider.ID, Message: "must be unique",
			}
		}
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			return sourcepayload.RecordReport{}, errors.WrapResource("decode", "provider", string(provider.ID), err)
		}
		providerIDs[string(provider.ID)] = struct{}{}
	}
	providerKeys := sortedRawKeys(payload.ProviderModels)
	report := sourcepayload.RecordReport{}
	for providerID := range providerIDs {
		if _, found := payload.ProviderModels[providerID]; !found {
			return sourcepayload.RecordReport{}, &errors.ValidationError{
				Field: "provider_models", Value: providerID, Message: "is required for every provider",
			}
		}
	}
	for _, providerID := range providerKeys {
		if _, found := providerIDs[providerID]; !found {
			return sourcepayload.RecordReport{}, &errors.ValidationError{
				Field: "provider_models", Value: providerID, Message: "references an unknown provider",
			}
		}
		remaining := remainingRecordBudget(report)
		models, recordReport, err := sourcepayload.DecodeJSONArray[Model](
			payload.ProviderModels[providerID],
			"provider_models["+providerID+"]",
			remaining,
		)
		if err != nil {
			return sourcepayload.RecordReport{}, err
		}
		mergeRecordReport(&report, recordReport)
		for _, model := range models {
			if err := builder.SetProviderModel(ProviderID(providerID), model); err != nil {
				report.Accepted--
				report.Rejected++
				report.Issues = append(report.Issues, sourcepayload.RecordIssue{
					Subject: "provider_models[" + providerID + "]/" + model.ID,
					Err:     errors.WrapResource("decode", "provider model", providerID+"/"+model.ID, err),
				})
			}
		}
	}
	return report, nil
}

func decodePayloadAuthors(
	builder *Builder,
	payload payloadEnvelope,
	providerReport sourcepayload.RecordReport,
) (sourcepayload.RecordReport, error) {
	authorIDs := make(map[string]struct{}, len(payload.Authors))
	for _, author := range payload.Authors {
		if _, exists := authorIDs[string(author.ID)]; exists {
			return sourcepayload.RecordReport{}, &errors.ValidationError{
				Field: "authors.id", Value: author.ID, Message: "must be unique",
			}
		}
		authorIDs[string(author.ID)] = struct{}{}
		if err := builder.SetAuthor(author); err != nil {
			return sourcepayload.RecordReport{}, errors.WrapResource("decode", "author", string(author.ID), err)
		}
	}
	authorKeys := sortedRawKeys(payload.AuthorModels)
	for authorID := range authorIDs {
		if _, found := payload.AuthorModels[authorID]; !found {
			return sourcepayload.RecordReport{}, &errors.ValidationError{
				Field: "author_models", Value: authorID, Message: "is required for every author",
			}
		}
	}
	report := sourcepayload.RecordReport{}
	for _, authorID := range authorKeys {
		if _, found := authorIDs[authorID]; !found {
			return sourcepayload.RecordReport{}, &errors.ValidationError{
				Field: "author_models", Value: authorID, Message: "references an unknown author",
			}
		}
		remaining := resourcepolicy.MaxModels -
			providerReport.Accepted - providerReport.Rejected -
			report.Accepted - report.Rejected
		if remaining < 0 {
			remaining = 0
		}
		models, recordReport, err := sourcepayload.DecodeJSONArray[Model](
			payload.AuthorModels[authorID],
			"author_models["+authorID+"]",
			remaining,
		)
		if err != nil {
			return sourcepayload.RecordReport{}, err
		}
		mergeRecordReport(&report, recordReport)
		for _, model := range models {
			if err := builder.SetAuthorModel(AuthorID(authorID), model); err != nil {
				report.Accepted--
				report.Rejected++
				report.Issues = append(report.Issues, sourcepayload.RecordIssue{
					Subject: "author_models[" + authorID + "]/" + model.ID,
					Err:     errors.WrapResource("decode", "authored model", authorID+"/"+model.ID, err),
				})
			}
		}
	}
	return report, nil
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
	if used >= resourcepolicy.MaxModels {
		return 0
	}
	return resourcepolicy.MaxModels - used
}
