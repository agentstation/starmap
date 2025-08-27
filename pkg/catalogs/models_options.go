package catalogs

import "maps"

// ModelsOption defines a function that configures a Models instance.
type ModelsOption func(*Models)

// WithModelsCapacity sets the initial capacity of the models map.
func WithModelsCapacity(capacity int) ModelsOption {
	return func(m *Models) {
		m.models = make(map[string]*Model, capacity)
	}
}

// WithModelsMap initializes the map with existing models.
func WithModelsMap(models map[string]*Model) ModelsOption {
	return func(m *Models) {
		if models != nil {
			m.models = make(map[string]*Model, len(models))
			maps.Copy(m.models, models)
		}
	}
}
