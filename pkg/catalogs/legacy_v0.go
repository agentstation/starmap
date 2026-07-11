package catalogs

import "github.com/agentstation/starmap/pkg/errors"

const (
	// LegacyCatalogSchemaVersion identifies the pre-definition/offering bare-ID
	// catalog API exposed by LegacyCatalogV0.
	LegacyCatalogSchemaVersion uint64 = 0
)

// LegacyCatalogV0 is the explicit transition adapter for the pre-split catalog
// read shape. New consumers should use Catalog.Definition, Catalog.Offering,
// and Catalog.ProviderOfferings.
type LegacyCatalogV0 struct {
	catalog *Catalog
}

// LegacyV0 returns the concrete schema-v0 compatibility adapter.
func (r *Catalog) LegacyV0() LegacyCatalogV0 {
	return LegacyCatalogV0{catalog: r}
}

// Models returns the legacy flattened bare-ID model collection.
func (a LegacyCatalogV0) Models() ModelsReader {
	return a.catalog.models
}

// ProviderModels returns legacy model records scoped to a provider or alias.
func (a LegacyCatalogV0) ProviderModels(id ProviderID) (ModelsReader, error) {
	models, ok := a.catalog.providerModels[id]
	if !ok {
		return nil, &errors.NotFoundError{Resource: "provider", ID: string(id)}
	}
	return models, nil
}

// ProviderModel returns one legacy provider-scoped model record.
func (a LegacyCatalogV0) ProviderModel(providerID ProviderID, modelID string) (Model, error) {
	models, err := a.ProviderModels(providerID)
	if err != nil {
		return Model{}, err
	}
	model, ok := models.Get(modelID)
	if !ok || model == nil {
		return Model{}, &errors.NotFoundError{
			Resource: "provider model",
			ID:       string(providerID) + "/" + modelID,
		}
	}
	return *model, nil
}

// FindModel returns one legacy flattened bare-ID model record.
func (a LegacyCatalogV0) FindModel(id string) (Model, error) {
	model, ok := a.catalog.models.Get(id)
	if !ok || model == nil {
		return Model{}, &errors.NotFoundError{Resource: "model", ID: id}
	}
	return *model, nil
}
