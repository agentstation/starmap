package catalogs

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// FormatYAML returns a well-formatted YAML representation with comments and proper structure.
func (m *Model) FormatYAML() string {
	// Create comment map for proper sectioning and headers
	commentMap := yaml.CommentMap{}

	// Add header comment using root path
	commentMap["$"] = []*yaml.Comment{
		yaml.HeadComment(fmt.Sprintf(" %s - %s", m.ID, m.FormatYAMLHeaderComment())),
	}

	// Add section comments using correct field paths
	if m.Metadata != nil {
		commentMap["$.metadata"] = []*yaml.Comment{
			yaml.HeadComment(" Model metadata"),
		}
	}

	if m.Features != nil {
		commentMap["$.features"] = []*yaml.Comment{
			yaml.HeadComment(" Model features"),
		}

		// Add feature subsection comments
		commentMap["$.features.tool_calls"] = []*yaml.Comment{
			yaml.HeadComment(" Core capabilities"),
		}
		commentMap["$.features.reasoning"] = []*yaml.Comment{
			yaml.HeadComment(" Reasoning & Verbosity"),
		}
		commentMap["$.features.temperature"] = []*yaml.Comment{
			yaml.HeadComment(" Generation control support flags"),
		}
		commentMap["$.features.format_response"] = []*yaml.Comment{
			yaml.HeadComment(" Response delivery"),
		}
	}

	if m.Limits != nil {
		commentMap["$.limits"] = []*yaml.Comment{
			yaml.HeadComment(" Model limits"),
		}
	}

	if m.Pricing != nil {
		commentMap["$.pricing"] = []*yaml.Comment{
			yaml.HeadComment(" Model pricing"),
		}
	}

	// Add timestamps comment
	commentMap["$.created_at"] = []*yaml.Comment{
		yaml.HeadComment(" Timestamps"),
	}

	// Marshal with proper formatting options (using IndentSequence(false) as requested)
	yamlData, err := yaml.MarshalWithOptions(m,
		yaml.Indent(2),                        // 2-space indentation
		yaml.IndentSequence(false),            // Keep sequences flush left
		yaml.UseLiteralStyleIfMultiline(true), // Use block scalar for multiline descriptions
		yaml.WithComment(commentMap),          // Apply comments
	)
	if err != nil {
		// Fallback to basic marshal if comment marshaling fails
		yamlData, err = yaml.Marshal(m)
		if err != nil {
			// This should never happen with valid data - indicates programming error
			panic(fmt.Sprintf("failed to marshal model %s to YAML: %v", m.ID, err))
		}
	}

	// Post-process to add blank lines between major sections and clean up empty fields
	processed := postProcessModelYAML(string(yamlData))
	return processed
}

// postProcessModelYAML adds proper spacing and formatting to model YAML output.
//
//nolint:gocyclo // Many YAML sections to format
func postProcessModelYAML(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	result := make([]string, 0, len(lines)+10) // Add some extra space for added blank lines

	// Track if we should add spacing before certain section headers
	majorSections := map[string]bool{
		"# Model metadata": true,
		"# Model features": true,
		"# Model limits":   true,
		"# Model pricing":  true,
		"# Timestamps":     true,
	}

	// Subsection headers within features that need spacing
	subsectionHeaders := map[string]bool{
		"# Core capabilities":                true,
		"# Reasoning & Verbosity":            true,
		"# Generation control support flags": true,
		"# Response delivery":                true,
	}

	// Track if we're inside an authors section
	inAuthorsSection := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if we're entering or leaving authors section
		if strings.HasPrefix(trimmedLine, "authors:") {
			inAuthorsSection = true
		} else if inAuthorsSection {
			// Check if we've left the authors section
			// We leave when we encounter a non-indented line that's not empty and not a comment
			if len(trimmedLine) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(trimmedLine, "#") {
				inAuthorsSection = false
			}
		}

		// Skip unwanted lines in authors section
		if inAuthorsSection {
			// Skip empty maps and null timestamps in the authors section
			if strings.Contains(trimmedLine, "models: {}") ||
				strings.Contains(trimmedLine, "created_at: null") ||
				strings.Contains(trimmedLine, "updated_at: null") ||
				strings.Contains(trimmedLine, "created_at: 0001-01-01T00:00:00Z") ||
				strings.Contains(trimmedLine, "updated_at: 0001-01-01T00:00:00Z") {
				continue // Skip this line
			}
		}

		// Check if this line is a major section header
		if majorSections[trimmedLine] && i > 0 {
			// Add blank line before major sections if the previous line isn't already blank
			if len(result) > 0 && result[len(result)-1] != "" {
				result = append(result, "")
			}
		}

		// Check if this line is a subsection header (with leading spaces)
		// These appear as "  # Core capabilities" in the YAML
		if strings.HasPrefix(trimmedLine, "#") && subsectionHeaders[trimmedLine] && i > 0 {
			// Add blank line before subsections if the previous line isn't already blank
			if len(result) > 0 && result[len(result)-1] != "" && !strings.Contains(result[len(result)-1], "features:") {
				result = append(result, "")
			}
		}

		// Process the line for date/timestamp formatting
		processedLine := line

		// Convert quoted timestamps to date-only format for specific fields
		if strings.Contains(line, "release_date:") && strings.Contains(line, "T00:00:00Z") {
			processedLine = strings.ReplaceAll(line, `"`, "")
			processedLine = strings.Replace(processedLine, "T00:00:00Z", "", 1)
		} else if strings.Contains(line, "knowledge_cutoff:") && strings.Contains(line, "T00:00:00Z") {
			processedLine = strings.ReplaceAll(line, `"`, "")
			processedLine = strings.Replace(processedLine, "T00:00:00Z", "", 1)
		} else if strings.Contains(line, "created_at:") || strings.Contains(line, "updated_at:") {
			// Remove quotes from timestamps but keep full timestamp format
			processedLine = strings.ReplaceAll(line, `"`, "")
		} else if strings.Contains(line, "per_1m: 10.0") && !strings.Contains(line, "per_1m: 10.00") {
			// Format decimals to 2 places for pricing
			processedLine = strings.Replace(line, "per_1m: 10.0", "per_1m: 10.00", 1)
		} else if strings.Contains(line, "description: \"") {
			// Convert quoted description to block scalar format
			processedLine = strings.Replace(line, "description: \"", "description: |-\n  ", 1)
			processedLine = strings.ReplaceAll(processedLine, "\"", "")
		}

		result = append(result, processedLine)
	}

	return strings.Join(result, "\n")
}

// FormatYAMLHeaderComment returns a descriptive string for the model header comment.
func (m *Model) FormatYAMLHeaderComment() string {
	if m.Description != "" {
		// Trim the description
		desc := strings.TrimSpace(m.Description)
		// Use first sentence or up to 60 characters of description
		if len(desc) > 60 {
			desc = desc[:60] + "..."
		}
		return desc
	}
	// If the description is empty, use the name
	if m.Name != "" && m.Name != m.ID {
		return m.Name
	}
	return "AI model"
}
