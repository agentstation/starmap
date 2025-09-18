package catalogs

// DeepCopyProviderModels creates a deep copy of a provider's Models map.
// Returns nil if the input map is nil.
func DeepCopyProviderModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		if v != nil {
			// Create a new Model instance with the same data
			modelCopy := *v
			result[k] = &modelCopy
		} else {
			result[k] = nil
		}
	}
	return result
}

// DeepCopyAuthorModels creates a deep copy of an author's Models map.
// Returns nil if the input map is nil.
func DeepCopyAuthorModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		if v != nil {
			// Create a new Model instance with the same data
			modelCopy := *v
			result[k] = &modelCopy
		} else {
			result[k] = nil
		}
	}
	return result
}

// DeepCopyProvider creates a deep copy of a Provider including its Models map.
func DeepCopyProvider(provider Provider) Provider {
	providerCopy := provider
	providerCopy.Models = DeepCopyProviderModels(provider.Models)
	return providerCopy
}

// DeepCopyAuthor creates a deep copy of an Author including its Models map.
func DeepCopyAuthor(author Author) Author {
	authorCopy := author
	authorCopy.Models = DeepCopyAuthorModels(author.Models)
	return authorCopy
}

// ShallowCopyProviderModels creates a shallow copy of a provider's Models map.
// The map is copied but Model pointers are shared.
// Returns nil if the input map is nil.
func ShallowCopyProviderModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		result[k] = v
	}
	return result
}

// ShallowCopyAuthorModels creates a shallow copy of an author's Models map.
// The map is copied but Model pointers are shared.
// Returns nil if the input map is nil.
func ShallowCopyAuthorModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		result[k] = v
	}
	return result
}
