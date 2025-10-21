package types

// ResourceType identifies the type of resource being tracked or merged in the catalog system.
// This allows provenance tracking and reconciliation to handle different resource types
// (models, providers, authors) with appropriate logic.
type ResourceType string

const (
	// ResourceTypeModel represents a model resource (e.g., gpt-4, claude-3).
	ResourceTypeModel ResourceType = "model"

	// ResourceTypeProvider represents a provider resource (e.g., openai, anthropic).
	ResourceTypeProvider ResourceType = "provider"

	// ResourceTypeAuthor represents an author resource (e.g., openai, meta).
	ResourceTypeAuthor ResourceType = "author"
)

// String returns the string representation of a resource type.
func (rt ResourceType) String() string {
	return string(rt)
}
