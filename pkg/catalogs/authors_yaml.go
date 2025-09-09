package catalogs

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatYAML returns the authors as formatted YAML with header comments, sections, and proper structure.
func (a *Authors) FormatYAML() string {
	if a == nil {
		return ""
	}

	authors := a.List()
	if len(authors) == 0 {
		return ""
	}

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

	// Group authors by category for organized sections
	grouped := groupAuthorsByCategory(authors)

	// Add authors in organized sections
	addAuthorSection(sequenceNode, "Major AI Companies", grouped["major"])
	addAuthorSection(sequenceNode, "Open Source & Academia", grouped["academic"])
	addAuthorSection(sequenceNode, "Research Institutions", grouped["research"])
	addAuthorSection(sequenceNode, "Cloud Providers", grouped["cloud"])
	addAuthorSection(sequenceNode, "Specialized Providers", grouped["specialized"])
	addAuthorSection(sequenceNode, "Other Organizations", grouped["other"])

	// Convert to string
	var sb strings.Builder
	encoder := yaml.NewEncoder(&sb)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc.Content[0]); err != nil {
		// Fallback to basic marshaling if formatting fails
		return a.fallbackYAML(authors)
	}
	_ = encoder.Close() // Ignoring close error for string builder

	return sb.String()
}

// buildHeaderComment creates the header comment for the authors.yaml file.
func buildHeaderComment() string {
	return `Known model authors and organizations with their metadata and social links
This file contains the complete author information that can be loaded at runtime`
}

// groupAuthorsByCategory organizes authors into logical sections.
func groupAuthorsByCategory(authors []*Author) map[string][]*Author {
	groups := map[string][]*Author{
		"major":       {},
		"academic":    {},
		"research":    {},
		"cloud":       {},
		"specialized": {},
		"other":       {},
	}

	// Define major AI companies
	majorCompanies := map[AuthorID]bool{
		AuthorIDOpenAI:    true,
		AuthorIDAnthropic: true,
		AuthorIDGoogle:    true,
		AuthorIDMeta:      true,
		AuthorIDMicrosoft: true,
		AuthorIDBaidu:     true,
		AuthorIDAlibabaQwen: true,
		"deepseek":       true,
		"cerebras":       true,
	}

	// Define academic/research institutions
	academicInstitutions := map[AuthorID]bool{
		"stanford":           true,
		"mit":                true,
		"berkeley":           true,
		"carnegie-mellon":    true,
		"nvidia":             true,
		"allen-institute":    true,
		"bigscience":         true,
		"eleutherai":         true,
		"together":           true,
		"hugginface":         true,
		"alignment-research": true,
	}

	// Define research institutions
	researchInstitutions := map[AuthorID]bool{
		"baai":           true,
		"tsinghua":       true,
		"peking":         true,
		"institute":      true,
		"lab":            true,
		"research":       true,
		"university":     true,
		"academic":       true,
		"mistral":        true,
		"stability":      true,
		"adept":          true,
		"cohere":         true,
	}

	// Define cloud providers
	cloudProviders := map[AuthorID]bool{
		"amazon":  true,
		"aws":     true,
		"azure":   true,
		"ibm":     true,
		"oracle":  true,
		"groq":    true,
		"fireworks": true,
	}

	// Categorize each author
	for _, author := range authors {
		authorID := strings.ToLower(string(author.ID))
		
		if majorCompanies[author.ID] {
			groups["major"] = append(groups["major"], author)
		} else if academicInstitutions[author.ID] {
			groups["academic"] = append(groups["academic"], author)
		} else if researchInstitutions[author.ID] || containsKeyword(authorID, []string{"institute", "lab", "research", "university", "academic"}) {
			groups["research"] = append(groups["research"], author)
		} else if cloudProviders[author.ID] || containsKeyword(authorID, []string{"cloud", "aws", "azure"}) {
			groups["cloud"] = append(groups["cloud"], author)
		} else if containsKeyword(authorID, []string{"ai", "ml", "tech"}) {
			groups["specialized"] = append(groups["specialized"], author)
		} else {
			groups["other"] = append(groups["other"], author)
		}
	}

	// Sort each group alphabetically
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].ID < group[j].ID
		})
	}

	return groups
}

// containsKeyword checks if any keyword is contained in the text.
func containsKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// addAuthorSection adds a section of authors with a comment header.
func addAuthorSection(sequenceNode *yaml.Node, sectionName string, authors []*Author) {
	if len(authors) == 0 {
		return
	}

	for i, author := range authors {
		authorNode := authorToYAMLNode(author)
		
		// Add section comment to the first author in each section
		if i == 0 {
			authorNode.HeadComment = fmt.Sprintf(" %s", sectionName)
		}
		
		sequenceNode.Content = append(sequenceNode.Content, authorNode)
	}
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