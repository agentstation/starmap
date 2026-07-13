package catalogstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// CatalogPayload is the canonical catalog schema-v2 payload.
type CatalogPayload = catalogs.CatalogPayload

// EncodeCatalogPayload deterministically encodes a readable catalog as schema v2.
func EncodeCatalogPayload(reader catalogs.Reader) ([]byte, error) {
	return catalogs.EncodeCatalogPayload(reader)
}

// DecodeCatalogPayload strictly decodes the sole current schema-v2 payload.
func DecodeCatalogPayload(data []byte) (*catalogs.Catalog, error) {
	var required map[string]json.RawMessage
	if err := json.Unmarshal(data, &required); err != nil {
		return nil, &errors.ParseError{Format: "json", File: "catalog payload", Message: err.Error(), Err: err}
	}
	for _, field := range []string{
		"schema_version", "providers", "authors", "endpoints",
		"definitions", "offerings", "provenance",
	} {
		if _, found := required[field]; !found {
			return nil, &errors.ValidationError{Field: field, Message: "is required"}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload CatalogPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, &errors.ParseError{Format: "json", File: "catalog payload", Message: err.Error(), Err: err}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, &errors.ParseError{Format: "json", File: "catalog payload", Message: "invalid trailing JSON", Err: err}
	}
	if payload.SchemaVersion != catalogs.CurrentCatalogSchemaVersion {
		return nil, &errors.ValidationError{
			Field:   "schema_version",
			Value:   payload.SchemaVersion,
			Message: fmt.Sprintf("must be exactly %d", catalogs.CurrentCatalogSchemaVersion),
		}
	}

	builder := catalogs.NewEmpty()
	for _, provider := range payload.Providers {
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			return nil, errors.WrapResource("decode", "provider", string(provider.ID), err)
		}
	}
	for _, author := range payload.Authors {
		author.Models = nil
		if err := builder.SetAuthor(author); err != nil {
			return nil, errors.WrapResource("decode", "author", string(author.ID), err)
		}
	}
	for _, endpoint := range payload.Endpoints {
		if err := builder.SetEndpoint(endpoint); err != nil {
			return nil, errors.WrapResource("decode", "endpoint", endpoint.ID, err)
		}
	}
	builder.SetProvenance(payload.Provenance)
	definitionIDs := make(map[catalogs.ModelDefinitionID]struct{}, len(payload.Definitions))
	for _, definition := range payload.Definitions {
		if _, found := definitionIDs[definition.ID]; found {
			return nil, &errors.ValidationError{Field: "definitions.id", Value: definition.ID, Message: "must be unique"}
		}
		definitionIDs[definition.ID] = struct{}{}
		if err := builder.SetDefinition(definition); err != nil {
			return nil, errors.WrapResource("decode", "model definition", string(definition.ID), err)
		}
	}
	offeringKeys := make(map[catalogs.OfferingKey]struct{}, len(payload.Offerings))
	for _, offering := range payload.Offerings {
		if _, found := offeringKeys[offering.Key()]; found {
			return nil, &errors.ValidationError{Field: "offerings.key", Value: offering.Key(), Message: "must be unique"}
		}
		offeringKeys[offering.Key()] = struct{}{}
		if err := builder.SetOffering(offering); err != nil {
			return nil, errors.WrapResource("decode", "provider offering", string(offering.ProviderID)+"/"+string(offering.ProviderModelID), err)
		}
	}
	return builder.Build()
}
