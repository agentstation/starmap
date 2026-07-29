package convert

import (
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// OpenAIModel represents a model in OpenAI API format.
// Field order matches the OpenAI API response schema.
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIModelsResponse represents the root response object for OpenAI models list API.
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// ToOpenAIModel converts a Model to OpenAI format.
func ToOpenAIModel(m *catalogs.Model) OpenAIModel {
	ownedBy := "system"

	// Collect all author IDs
	if len(m.Authors) > 0 {
		var authorIDs []string
		for _, author := range m.Authors {
			if author.ID != "" {
				authorIDs = append(authorIDs, author.ID.String())
			}
		}
		if len(authorIDs) > 0 {
			ownedBy = strings.Join(authorIDs, ",")
		}
	}

	return OpenAIModel{
		ID:      m.ID,
		Object:  "model",
		Created: m.CreatedAt.Unix(),
		OwnedBy: ownedBy,
	}
}
