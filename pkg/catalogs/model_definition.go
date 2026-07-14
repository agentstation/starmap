package catalogs

import (
	"fmt"
	"strings"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/errors"
)

// ModelDefinition is the canonical provider-independent description of a model.
// Provider service facts belong to ProviderOffering, never this record.
type ModelDefinition struct {
	ID           ModelDefinitionID           `json:"id" yaml:"id"`
	Name         string                      `json:"name" yaml:"name"`
	AuthorIDs    []AuthorID                  `json:"author_ids" yaml:"author_ids"`
	Description  string                      `json:"description,omitempty" yaml:"description,omitempty"`
	Metadata     ModelDefinitionMetadata     `json:"metadata" yaml:"metadata"`
	Lineage      ModelDefinitionLineage      `json:"lineage" yaml:"lineage"`
	Weights      ModelDefinitionWeights      `json:"weights" yaml:"weights"`
	Capabilities ModelDefinitionCapabilities `json:"capabilities" yaml:"capabilities"`
	CreatedAt    utc.Time                    `json:"created_at" yaml:"created_at"`
	UpdatedAt    utc.Time                    `json:"updated_at" yaml:"updated_at"`
}

// ModelDefinitionMetadata contains provider-independent release and discovery metadata.
type ModelDefinitionMetadata struct {
	ReleaseDate     utc.Time   `json:"release_date" yaml:"release_date"`
	KnowledgeCutoff *utc.Time  `json:"knowledge_cutoff,omitempty" yaml:"knowledge_cutoff,omitempty"`
	Tags            []ModelTag `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// ModelDefinitionLineage describes canonical model-family relationships.
type ModelDefinitionLineage struct {
	Family string             `json:"family,omitempty" yaml:"family,omitempty"`
	Root   *ModelDefinitionID `json:"root,omitempty" yaml:"root,omitempty"`
	Parent *ModelDefinitionID `json:"parent,omitempty" yaml:"parent,omitempty"`
}

// ModelDefinitionWeights describes provider-independent model weights and architecture.
type ModelDefinitionWeights struct {
	Open         bool               `json:"open" yaml:"open"`
	Architecture *ModelArchitecture `json:"architecture,omitempty" yaml:"architecture,omitempty"`
}

// ModelDefinitionCapabilities groups intrinsic model behavior independently of
// any provider's service limits, price, endpoint, or availability.
type ModelDefinitionCapabilities struct {
	Features        *ModelFeatures      `json:"features,omitempty" yaml:"features,omitempty"`
	Attachments     *ModelAttachments   `json:"attachments,omitempty" yaml:"attachments,omitempty"`
	Generation      *ModelGeneration    `json:"generation,omitempty" yaml:"generation,omitempty"`
	Reasoning       *ModelControlLevels `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	ReasoningTokens *IntRange           `json:"reasoning_tokens,omitempty" yaml:"reasoning_tokens,omitempty"`
	Verbosity       *ModelControlLevels `json:"verbosity,omitempty" yaml:"verbosity,omitempty"`
	Tools           *ModelTools         `json:"tools,omitempty" yaml:"tools,omitempty"`
	Delivery        *ModelDelivery      `json:"delivery,omitempty" yaml:"delivery,omitempty"`
}

// Validate verifies canonical identity and authorship invariants.
func (d ModelDefinition) Validate() error {
	if strings.TrimSpace(string(d.ID)) == "" {
		return definitionValidationError("id", d.ID, validationMessageIsRequired)
	}
	if strings.TrimSpace(d.Name) == "" {
		return definitionValidationError("name", d.Name, validationMessageIsRequired)
	}
	seenAuthors := make(map[AuthorID]struct{}, len(d.AuthorIDs))
	for index, authorID := range d.AuthorIDs {
		if strings.TrimSpace(string(authorID)) == "" {
			return definitionValidationError(fmt.Sprintf("author_ids[%d]", index), authorID, validationMessageMustNotBeEmpty)
		}
		if _, exists := seenAuthors[authorID]; exists {
			return definitionValidationError(fmt.Sprintf("author_ids[%d]", index), authorID, validationMessageMustBeUnique)
		}
		seenAuthors[authorID] = struct{}{}
	}
	if d.Lineage.Root != nil && *d.Lineage.Root == d.ID {
		return definitionValidationError("lineage.root", *d.Lineage.Root, "must not reference the definition itself")
	}
	if d.Lineage.Parent != nil && *d.Lineage.Parent == d.ID {
		return definitionValidationError("lineage.parent", *d.Lineage.Parent, "must not reference the definition itself")
	}
	return nil
}

func definitionValidationError(field string, value any, message string) error {
	return &errors.ValidationError{Field: field, Value: value, Message: message}
}

func copyModelDefinition(definition ModelDefinition) ModelDefinition {
	copyDefinition := definition
	copyDefinition.AuthorIDs = append([]AuthorID(nil), definition.AuthorIDs...)
	copyDefinition.Metadata.Tags = append([]ModelTag(nil), definition.Metadata.Tags...)
	if definition.Metadata.KnowledgeCutoff != nil {
		knowledgeCutoff := *definition.Metadata.KnowledgeCutoff
		copyDefinition.Metadata.KnowledgeCutoff = &knowledgeCutoff
	}
	if definition.Lineage.Root != nil {
		root := *definition.Lineage.Root
		copyDefinition.Lineage.Root = &root
	}
	if definition.Lineage.Parent != nil {
		parent := *definition.Lineage.Parent
		copyDefinition.Lineage.Parent = &parent
	}
	copyDefinition.Weights.Architecture = deepCopyModelArchitecture(definition.Weights.Architecture)
	copyDefinition.Capabilities.Features = deepCopyModelFeatures(definition.Capabilities.Features)
	copyDefinition.Capabilities.Attachments = deepCopyModelAttachments(definition.Capabilities.Attachments)
	copyDefinition.Capabilities.Generation = deepCopyModelGeneration(definition.Capabilities.Generation)
	copyDefinition.Capabilities.Reasoning = deepCopyModelControlLevels(definition.Capabilities.Reasoning)
	if definition.Capabilities.ReasoningTokens != nil {
		reasoningTokens := *definition.Capabilities.ReasoningTokens
		copyDefinition.Capabilities.ReasoningTokens = &reasoningTokens
	}
	copyDefinition.Capabilities.Verbosity = deepCopyModelControlLevels(definition.Capabilities.Verbosity)
	copyDefinition.Capabilities.Tools = deepCopyModelTools(definition.Capabilities.Tools)
	copyDefinition.Capabilities.Delivery = deepCopyModelDelivery(definition.Capabilities.Delivery)
	return copyDefinition
}
