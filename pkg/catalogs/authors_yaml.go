package catalogs

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
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

	// Sort authors alphabetically by ID
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].ID < authors[j].ID
	})

	// Create YAML document with header comment
	doc := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{
				Kind:        yaml.SequenceNode,
				HeadComment: buildHeaderComment(),
				Content:     []*yaml.Node{},
			},
		},
	}

	sequenceNode := doc.Content[0]

	// Add all authors in alphabetical order
	for _, author := range authors {
		authorNode := authorToYAMLNode(author)
		sequenceNode.Content = append(sequenceNode.Content, authorNode)
	}

	// Convert to string
	var sb strings.Builder
	encoder := yaml.NewEncoder(&sb)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc.Content[0]); err != nil {
		// Fallback to basic marshaling if formatting fails
		return a.fallbackYAML(authors)
	}
	_ = encoder.Close() // Ignoring close error for string builder

	// Post-process the YAML to add blank lines between entries
	yamlContent := sb.String()
	return a.addProperSpacing(yamlContent)
}

// buildHeaderComment creates the header comment for the authors.yaml file.
func buildHeaderComment() string {
	return `Known model authors and organizations with their metadata and social links
This file contains the complete author information that can be loaded at runtime

`
}


// authorToYAMLNode converts an Author to a YAML node with proper field ordering.
func authorToYAMLNode(author *Author) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{},
	}

	// Add fields in the desired order, excluding unwanted fields
	addStringField(node, "id", string(author.ID))
	addStringField(node, "name", author.Name)

	// Add aliases if present
	if len(author.Aliases) > 0 {
		aliasesNode := &yaml.Node{Kind: yaml.SequenceNode}
		for _, alias := range author.Aliases {
			aliasesNode.Content = append(aliasesNode.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: string(alias),
			})
		}
		addFieldNode(node, "aliases", aliasesNode)
	}

	// Add description if present
	if author.Description != nil {
		addStringField(node, "description", *author.Description)
	}

	// Add URLs if present
	if author.Website != nil {
		addStringField(node, "website", *author.Website)
	}
	if author.HuggingFace != nil {
		addStringField(node, "huggingface", *author.HuggingFace)
	}
	if author.GitHub != nil {
		addStringField(node, "github", *author.GitHub)
	}
	if author.Twitter != nil {
		addStringField(node, "twitter", *author.Twitter)
	}

	// Add catalog if present
	if author.Catalog != nil {
		catalogNode := &yaml.Node{Kind: yaml.MappingNode}
		addStringField(catalogNode, "provider_id", string(author.Catalog.ProviderID))
		
		if len(author.Catalog.Patterns) > 0 {
			patternsNode := &yaml.Node{Kind: yaml.SequenceNode}
			for _, pattern := range author.Catalog.Patterns {
				patternsNode.Content = append(patternsNode.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: pattern,
				})
			}
			addFieldNode(catalogNode, "patterns", patternsNode)
		}
		
		if author.Catalog.Description != nil {
			addStringField(catalogNode, "description", *author.Catalog.Description)
		}
		
		addFieldNode(node, "catalog", catalogNode)
	}

	return node
}

// addStringField adds a string field to a YAML mapping node.
func addStringField(node *yaml.Node, key, value string) {
	if value == "" {
		return
	}
	
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valueNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	
	node.Content = append(node.Content, keyNode, valueNode)
}

// addFieldNode adds a field node to a YAML mapping node.
func addFieldNode(node *yaml.Node, key string, valueNode *yaml.Node) {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	node.Content = append(node.Content, keyNode, valueNode)
}

// fallbackYAML provides a fallback YAML generation method.
func (a *Authors) fallbackYAML(authors []*Author) string {
	// Clean authors - remove unwanted fields
	cleaned := make([]map[string]interface{}, 0, len(authors))
	
	for _, author := range authors {
		entry := map[string]interface{}{
			"id":   author.ID,
			"name": author.Name,
		}
		
		if len(author.Aliases) > 0 {
			entry["aliases"] = author.Aliases
		}
		
		if author.Description != nil {
			entry["description"] = *author.Description
		}
		
		if author.Website != nil {
			entry["website"] = *author.Website
		}
		
		if author.HuggingFace != nil {
			entry["huggingface"] = *author.HuggingFace
		}
		
		if author.GitHub != nil {
			entry["github"] = *author.GitHub
		}
		
		if author.Twitter != nil {
			entry["twitter"] = *author.Twitter
		}
		
		if author.Catalog != nil {
			entry["catalog"] = author.Catalog
		}
		
		cleaned = append(cleaned, entry)
	}

	data, err := yaml.Marshal(cleaned)
	if err != nil {
		return fmt.Sprintf("# Error formatting authors YAML: %v\n", err)
	}
	
	// Add header comment manually
	header := "# Known model authors and organizations with their metadata and social links\n# This file contains the complete author information that can be loaded at runtime\n\n"
	return header + string(data)
}

// addProperSpacing post-processes YAML content to add blank lines between author entries.
func (a *Authors) addProperSpacing(yamlContent string) string {
	lines := strings.Split(yamlContent, "\n")
	var result []string
	
	inEntry := false
	for i, line := range lines {
		result = append(result, line)
		
		// Track if we're in an author entry
		if strings.HasPrefix(line, "- id:") {
			inEntry = true
		} else if strings.HasPrefix(line, "#") || (strings.HasPrefix(line, "- id:") && inEntry) {
			inEntry = false
		}
		
		// Add blank line after author entries (before next entry or section header)
		if inEntry && i < len(lines)-1 {
			nextLine := lines[i+1]
			// If next line is start of new entry or section header, add blank line
			if strings.HasPrefix(nextLine, "- id:") || strings.HasPrefix(nextLine, "#") {
				result = append(result, "")
				inEntry = false
			}
		}
	}
	
	return strings.Join(result, "\n")
}