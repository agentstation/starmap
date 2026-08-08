package catalogs

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/errors"
)

// FormatYAML returns a well-formatted YAML representation with comments and proper structure.
func (m *Model) FormatYAML() string {
	formatted, _ := m.EncodeYAML()
	return formatted
}

// EncodeYAML returns formatted YAML or a typed parse error when model values
// cannot be represented safely.
func (m *Model) EncodeYAML() (string, error) {
	// Every human-editable model YAML exposes the complete Boolean capability
	// surface, including models for which no source supplied feature claims.
	// Use a shallow projection copy so formatting never mutates the model.
	projected := m
	if m.Features == nil {
		projectedModel := *m
		projectedModel.Features = &ModelFeatures{}
		projected = &projectedModel
	}

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

	if projected.Features != nil {
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

	// goccy/go-yaml's literal-block encoder doubles backslashes on every
	// encode/decode cycle. Force single-quoted scalars for affected models so
	// those descriptions round-trip exactly.
	var (
		yamlData []byte
		err      error
	)
	if strings.Contains(projected.Description, `\`) {
		yamlData, err = yaml.MarshalWithOptions(projected,
			yaml.Indent(2),
			yaml.IndentSequence(false),
			yaml.UseSingleQuote(true),
			yaml.WithComment(commentMap),
		)
	} else {
		yamlData, err = yaml.MarshalWithOptions(projected,
			yaml.Indent(2),
			yaml.IndentSequence(false),
			yaml.UseLiteralStyleIfMultiline(true),
			yaml.WithComment(commentMap),
		)
	}
	if err != nil {
		// Fallback to basic marshal if comment marshaling fails
		yamlData, err = yaml.Marshal(projected)
		if err != nil {
			return "", errors.WrapParse("yaml", "model "+m.ID, err)
		}
	}

	// Post-process to add blank lines between major sections and clean up empty fields
	processed := postProcessModelYAML(string(yamlData))
	return processed, nil
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
		} else if literal, ok := quotedDescriptionLiteral(line); ok {
			// Decode the YAML scalar before changing its style. Removing quote
			// characters directly leaves escape backslashes in the value.
			processedLine = literal
		}

		result = append(result, processedLine)
	}

	return strings.Join(result, "\n")
}

func quotedDescriptionLiteral(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, `description: "`) || trimmed == `description: ""` {
		return "", false
	}
	var document struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(trimmed+"\n"), &document); err != nil {
		return "", false
	}
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	lines := strings.Split(document.Description, "\n")
	var output strings.Builder
	output.WriteString(indent)
	output.WriteString("description: |-\n")
	for index, descriptionLine := range lines {
		if index > 0 {
			output.WriteByte('\n')
		}
		output.WriteString(indent)
		output.WriteString("  ")
		output.WriteString(descriptionLine)
	}
	return output.String(), true
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
