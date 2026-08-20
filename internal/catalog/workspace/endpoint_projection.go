package workspace

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	endpointProjectionFilename      = "endpoints.yaml"
	endpointProjectionSchemaVersion = 2
)

type endpointProjection struct {
	SchemaVersion int                       `yaml:"schema_version"`
	GenerationID  string                    `yaml:"generation_id"`
	CatalogDigest string                    `yaml:"catalog_digest"`
	Models        []endpointProjectionModel `yaml:"models"`
}

type endpointProjectionModel struct {
	Model     catalogs.ModelDefinitionID `yaml:"model"`
	Endpoints []endpointProjectionRow    `yaml:"endpoints"`
}

type endpointProjectionRow struct {
	ProviderID       catalogs.ProviderID                          `yaml:"provider"`
	ProviderModelID  catalogs.ProviderModelID                     `yaml:"provider_model_id"`
	Pricing          *catalogs.ModelPricing                       `yaml:"pricing,omitempty"`
	Limits           *catalogs.ModelLimits                        `yaml:"limits,omitempty"`
	Availability     catalogs.OfferingAvailability                `yaml:"availability"`
	Lifecycle        catalogs.OfferingLifecycle                   `yaml:"lifecycle"`
	Service          catalogs.ProviderOfferingServiceCapabilities `yaml:"service"`
	ServiceEndpoints []catalogs.ProviderOfferingEndpoint          `yaml:"service_endpoints,omitempty"`
	Modes            map[string]catalogs.ProviderOfferingMode     `yaml:"modes,omitempty"`
}

// EncodeEndpointProjection renders the deterministic, generation-bound
// author/model to provider-offering projection used by workspace tooling.
func EncodeEndpointProjection(catalog *catalogs.Catalog, identity Identity) ([]byte, error) {
	if catalog == nil {
		return nil, &errors.ValidationError{
			Field: "endpoint_projection.catalog", Message: "is required",
		}
	}
	if err := identity.Validate(); err != nil {
		return nil, err
	}

	grouped := make(map[catalogs.ModelDefinitionID][]endpointProjectionRow)
	for _, definition := range catalog.Definitions() {
		offerings, err := catalog.DefinitionOfferings(definition.ID)
		if err != nil {
			return nil, errors.WrapResource(
				"read", "model definition offerings", string(definition.ID), err,
			)
		}
		for _, offering := range offerings {
			grouped[offering.DefinitionID] = append(
				grouped[offering.DefinitionID],
				endpointProjectionRow{
					ProviderID:       offering.ProviderID,
					ProviderModelID:  offering.ProviderModelID,
					Pricing:          offering.Pricing,
					Limits:           offering.Limits,
					Availability:     offering.Availability,
					Lifecycle:        offering.Lifecycle,
					Service:          offering.Service,
					ServiceEndpoints: offering.Endpoints,
					Modes:            offering.Modes,
				},
			)
		}
	}

	modelIDs := make([]catalogs.ModelDefinitionID, 0, len(grouped))
	for modelID := range grouped {
		modelIDs = append(modelIDs, modelID)
	}
	slices.Sort(modelIDs)
	document := endpointProjection{
		SchemaVersion: endpointProjectionSchemaVersion,
		GenerationID:  identity.GenerationID,
		CatalogDigest: identity.PayloadChecksum,
		Models:        make([]endpointProjectionModel, 0, len(modelIDs)),
	}
	for _, modelID := range modelIDs {
		rows := grouped[modelID]
		slices.SortFunc(rows, func(left, right endpointProjectionRow) int {
			if compared := strings.Compare(string(left.ProviderID), string(right.ProviderID)); compared != 0 {
				return compared
			}
			return strings.Compare(string(left.ProviderModelID), string(right.ProviderModelID))
		})
		document.Models = append(document.Models, endpointProjectionModel{
			Model: modelID, Endpoints: rows,
		})
	}

	data, err := yaml.MarshalWithOptions(document, yaml.IndentSequence(true))
	if err != nil {
		return nil, errors.WrapParse("yaml", endpointProjectionFilename, err)
	}
	return data, nil
}

func writeEndpointProjection(path string, catalog *catalogs.Catalog, identity Identity) (string, error) {
	data, err := EncodeEndpointProjection(catalog, identity)
	if err != nil {
		return "", err
	}
	target := filepath.Join(path, endpointProjectionFilename)
	if err := os.WriteFile(target, data, fileMode); err != nil {
		return "", errors.WrapIO("write", target, err)
	}
	return endpointProjectionChecksum(data), nil
}

func readEndpointProjectionChecksum(path string) (string, error) {
	data, err := os.ReadFile( //nolint:gosec // Validated workspace path plus fixed managed filename.
		filepath.Join(path, endpointProjectionFilename),
	)
	if err != nil {
		return "", err
	}
	return endpointProjectionChecksum(data), nil
}

func endpointProjectionChecksum(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest[:])
}
