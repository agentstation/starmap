package catalogs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// FormatYAML returns the authors as formatted YAML sorted alphabetically by ID.
func (a *Authors) FormatYAML() string {
	if a == nil {
		return ""
	}

	authors := a.List()
	if len(authors) == 0 {
		return ""
	}

	return formatAuthorsYAML(authors)
}

func formatAuthorsYAML(authors []Author) string {
	// Sort authors alphabetically by ID
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].ID < authors[j].ID
	})

	// Create comment map for proper headers
	commentMap := yaml.CommentMap{}

	// Add header comment using root path
	commentMap["$"] = []*yaml.Comment{
		yaml.HeadComment(" Known model authors and organizations with their metadata and social links"),
		yaml.HeadComment(" This file contains the complete author information that can be loaded at runtime"),
	}

	// Add comments above each author entry using their name
	for i, author := range authors {
		path := fmt.Sprintf("$[%d]", i)
		commentMap[path] = []*yaml.Comment{
			yaml.HeadComment(" " + author.Name),
		}
	}

	// Let the library handle the formatting properly
	yamlData, err := yaml.MarshalWithOptions(authors,
		yaml.Indent(2),               // 2-space indentation
		yaml.IndentSequence(false),   // Keep root array flush left (no indentation)
		yaml.WithComment(commentMap), // Add comments
	)
	if err != nil {
		// Fallback to basic YAML if enhanced formatting fails
		basicYaml, _ := yaml.Marshal(authors)
		return string(basicYaml)
	}

	// Post-process to filter unwanted fields and add spacing between authors
	filtered := filterUnwantedFields(string(yamlData))
	return addBlankLinesBetweenAuthors(filtered)
}

// filterUnwantedFields removes unwanted YAML fields from authors.
func filterUnwantedFields(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip unwanted lines
		if strings.Contains(trimmedLine, "models: {}") ||
			strings.Contains(trimmedLine, "created_at: null") ||
			strings.Contains(trimmedLine, "updated_at: null") ||
			strings.Contains(trimmedLine, "created_at: 0001-01-01T00:00:00Z") ||
			strings.Contains(trimmedLine, "updated_at: 0001-01-01T00:00:00Z") {
			continue // Skip this line
		}

		// Keep the line
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// addBlankLinesBetweenAuthors adds spacing between author sections.
func addBlankLinesBetweenAuthors(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	result := make([]string, 0, len(lines)+len(lines)/10) // Add extra space for blank lines

	for i, line := range lines {
		// Add blank line before each author comment (except the first one)
		if strings.HasPrefix(line, "#") && i > 0 {
			result = append(result, "")
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
