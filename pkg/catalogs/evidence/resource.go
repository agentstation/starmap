package evidence

// ResourceType identifies a catalog resource category. Provenance and
// reconciliation use it to select logic for models, providers, and authors.
type ResourceType string

const (
	// ResourceTypeModel represents a model resource (e.g., gpt-4, claude-3).
	ResourceTypeModel ResourceType = "model"

	// ResourceTypeProvider represents a provider resource (e.g., openai, anthropic).
	ResourceTypeProvider ResourceType = "provider"

	// ResourceTypeAuthor represents an author resource (e.g., openai, meta).
	ResourceTypeAuthor ResourceType = "author"

	// ResourceTypeModelDefinition represents provider-independent canonical model facts.
	ResourceTypeModelDefinition ResourceType = "model_definition"

	// ResourceTypeProviderOffering represents one provider-specific model service contract.
	ResourceTypeProviderOffering ResourceType = "provider_offering"
)

// String returns text for resource type.
func (rt ResourceType) String() string {
	return string(rt)
}
