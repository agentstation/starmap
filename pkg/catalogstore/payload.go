package catalogstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// CatalogPayload is the canonical catalog schema-v1 payload.
type CatalogPayload = catalogs.CatalogPayload

// EncodeCatalogPayload deterministically encodes a readable catalog as schema v1.
func EncodeCatalogPayload(reader catalogs.Reader) ([]byte, error) {
	return catalogs.EncodeCatalogPayload(reader)
}

// DecodeCatalogPayload strictly decodes and publishes a schema-v1 catalog payload.
func DecodeCatalogPayload(data []byte) (*catalogs.Catalog, error) {
	var required map[string]json.RawMessage
	if err := json.Unmarshal(data, &required); err != nil {
		return nil, &errors.ParseError{Format: "json", File: "catalog payload", Message: err.Error(), Err: err}
	}
	for _, field := range []string{
		"schema_version", "providers", "authors", "endpoints",
		"provider_models", "author_models", "provenance",
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
			Message: fmt.Sprintf("must be %d", catalogs.CurrentCatalogSchemaVersion),
		}
	}

	builder := catalogs.NewEmpty()
	for _, provider := range payload.Providers {
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			return nil, errors.WrapResource("decode", "provider", string(provider.ID), err)
		}
	}
	for providerID, models := range payload.ProviderModels {
		for _, model := range models {
			if err := builder.SetProviderModel(catalogs.ProviderID(providerID), model); err != nil {
				return nil, errors.WrapResource("decode", "provider model", providerID+"/"+model.ID, err)
			}
		}
	}
	for _, author := range payload.Authors {
		author.Models = make(map[string]*catalogs.Model)
		for _, model := range payload.AuthorModels[string(author.ID)] {
			modelCopy := catalogs.DeepCopyModel(model)
			author.Models[model.ID] = &modelCopy
		}
		if err := builder.SetAuthor(author); err != nil {
			return nil, errors.WrapResource("decode", "author", string(author.ID), err)
		}
	}
	for authorID := range payload.AuthorModels {
		if _, err := builder.Author(catalogs.AuthorID(authorID)); err != nil {
			return nil, errors.WrapResource("decode", "author models", authorID, err)
		}
	}
	for _, endpoint := range payload.Endpoints {
		if err := builder.SetEndpoint(endpoint); err != nil {
			return nil, errors.WrapResource("decode", "endpoint", endpoint.ID, err)
		}
	}
	builder.SetProvenance(payload.Provenance)
	return builder.Build()
}
