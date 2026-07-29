package catalogs

import "github.com/agentstation/starmap/pkg/errors"

// testBuilderModels flattens provider-scoped models only for construction
// tests that are not exercising the public read API. Production
// callers must choose a provider model or publish an immutable Catalog.
func testBuilderModels(builder *Builder) []Model {
	models := NewModels()
	for _, provider := range builder.Providers().List() {
		for modelID, model := range provider.Models {
			if model == nil {
				continue
			}
			_ = models.Set(modelID, model)
		}
	}
	return models.List()
}

func testBuilderFindModel(builder *Builder, id string) (Model, error) {
	for _, model := range testBuilderModels(builder) {
		if model.ID == id {
			return model, nil
		}
	}
	return Model{}, &errors.NotFoundError{Resource: "provider model fixture", ID: id}
}
