package base

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/goccy/go-yaml"
)

// ParseModel parses a YAML model file and handles the complex authors field conversion
func ParseModel(data []byte, path string) (*catalogs.Model, error) {
	// Parse the YAML into a generic map first
	var rawData map[string]any
	if err := yaml.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("unmarshaling raw data from %s: %w", path, err)
	}

	// Handle authors field separately since it can be strings or structs
	var model catalogs.Model
	if authorsRaw, exists := rawData["authors"]; exists {
		// Save authors for special handling
		delete(rawData, "authors")

		// Parse the rest of the model
		modelData, _ := yaml.Marshal(rawData)
		if err := yaml.Unmarshal(modelData, &model); err != nil {
			return nil, fmt.Errorf("unmarshaling model from %s: %w", path, err)
		}

		// Handle authors
		authors, err := parseAuthors(authorsRaw)
		if err != nil {
			return nil, fmt.Errorf("parsing authors from %s: %w", path, err)
		}
		model.Authors = authors
	} else {
		// No authors field - just parse normally
		modelData, _ := yaml.Marshal(rawData)
		if err := yaml.Unmarshal(modelData, &model); err != nil {
			return nil, fmt.Errorf("unmarshaling model from %s: %w", path, err)
		}
	}

	if model.ID == "" {
		return nil, fmt.Errorf("model in %s has no ID", path)
	}

	return &model, nil
}

// parseAuthors converts the authors field from various formats to []catalogs.Author
func parseAuthors(authorsRaw any) ([]catalogs.Author, error) {
	authorsSlice, ok := authorsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("authors field is not a slice")
	}

	authors := make([]catalogs.Author, 0, len(authorsSlice))
	for i, a := range authorsSlice {
		switch v := a.(type) {
		case string:
			// Convert string to Author
			authors = append(authors, catalogs.Author{
				ID:   catalogs.AuthorID(v),
				Name: v,
			})
		case map[string]any:
			// Convert map to Author
			author, err := parseAuthorMap(v)
			if err != nil {
				return nil, fmt.Errorf("parsing author at index %d: %w", i, err)
			}
			authors = append(authors, author)
		default:
			return nil, fmt.Errorf("unsupported author type at index %d: %T", i, v)
		}
	}

	return authors, nil
}

// parseAuthorMap converts a map[string]any to catalogs.Author
func parseAuthorMap(authorMap map[string]any) (catalogs.Author, error) {
	var author catalogs.Author

	if id, ok := authorMap["id"].(string); ok {
		author.ID = catalogs.AuthorID(id)
	} else {
		return author, fmt.Errorf("author map missing or invalid 'id' field")
	}

	if name, ok := authorMap["name"].(string); ok {
		author.Name = name
	}

	// Add other fields as needed in the future
	// This centralizes the logic for expanding author parsing

	return author, nil
}
