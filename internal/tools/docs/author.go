package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// generateAuthorDocs generates documentation for all authors
func (g *Generator) generateAuthorDocs(dir string, catalog catalogs.Reader) error {
	authors := catalog.Authors().List()
	models := catalog.Models().List()

	// First generate the author index
	if err := g.generateAuthorIndex(dir, authors, models, catalog); err != nil {
		return fmt.Errorf("generating author index: %w", err)
	}

	// Then generate individual author pages
	for _, author := range authors {
		authorDir := filepath.Join(dir, string(author.ID))
		if err := os.MkdirAll(authorDir, constants.DirPermissions); err != nil {
			return fmt.Errorf("creating author directory: %w", err)
		}

		// Create models subdirectory for this author
		modelsDir := filepath.Join(authorDir, "models")
		if err := os.MkdirAll(modelsDir, constants.DirPermissions); err != nil {
			return fmt.Errorf("creating author models directory: %w", err)
		}

		if err := g.generateAuthorReadme(authorDir, author, catalog); err != nil {
			return fmt.Errorf("generating author %s README: %w", author.ID, err)
		}

		// Generate model pages for this author
		if err := g.generateAuthorModelPages(modelsDir, author, catalog); err != nil {
			return fmt.Errorf("generating author %s model pages: %w", author.ID, err)
		}
	}

	return nil
}

// generateAuthorIndex generates the main author listing page
func (g *Generator) generateAuthorIndex(dir string, authors []*catalogs.Author, models []*catalogs.Model, catalog catalogs.Reader) error {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating author directory: %w", err)
	}
	
	indexFile := filepath.Join(dir, "README.md")
	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("creating author index: %w", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "# 👥 Model Authors")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Organizations and researchers that develop and train AI models.")
	fmt.Fprintln(f)

	// Calculate model counts for each author
	type authorInfo struct {
		author     *catalogs.Author
		modelCount int
		providers  map[string]bool
	}

	authorInfoMap := make(map[catalogs.AuthorID]*authorInfo)
	for _, author := range authors {
		authorInfoMap[author.ID] = &authorInfo{
			author:    author,
			providers: make(map[string]bool),
		}
	}

	// Count models and track providers
	// Note: We'll need to track providers from the provider catalog
	providers := catalog.Providers().List()
	for _, model := range models {
		for _, ma := range model.Authors {
			if info, ok := authorInfoMap[ma.ID]; ok {
				info.modelCount++
				// Track which providers host this author's models
				for _, provider := range providers {
					if provider.Models != nil {
						if _, hasModel := provider.Models[model.ID]; hasModel {
							info.providers[string(provider.ID)] = true
						}
					}
				}
			}
		}
	}

	// Convert to slice and sort by model count
	var authorInfos []*authorInfo
	for _, info := range authorInfoMap {
		authorInfos = append(authorInfos, info)
	}
	sort.Slice(authorInfos, func(i, j int) bool {
		return authorInfos[i].modelCount > authorInfos[j].modelCount
	})

	// Author comparison table
	fmt.Fprintln(f, "## Author Overview")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Author | Models | Hosted By | Focus Area |")
	fmt.Fprintln(f, "|--------|--------|-----------|------------|")

	for _, info := range authorInfos {
		author := info.author

		// Get provider list
		var providerList []string
		for p := range info.providers {
			providerList = append(providerList, p)
		}
		sort.Strings(providerList)
		providersStr := "—"
		if len(providerList) > 0 {
			if len(providerList) > 3 {
				providersStr = fmt.Sprintf("%d providers", len(providerList))
			} else {
				providersStr = ""
				for i, p := range providerList {
					if i > 0 {
						providersStr += ", "
					}
					providersStr += p
				}
			}
		}

		focusArea := getFocusArea(author)
		badge := getAuthorBadge(author.Name)

		fmt.Fprintf(f, "| %s **[%s](%s/)** | %d | %s | %s |\n",
			badge, author.Name, string(author.ID), info.modelCount, providersStr, focusArea)
	}

	fmt.Fprintln(f)

	// Categories section
	fmt.Fprintln(f, "## By Category")
	fmt.Fprintln(f)

	categories := map[string][]*authorInfo{
		"🏢 Major Tech Companies":  {},
		"🚀 AI Startups":            {},
		"🎓 Research Organizations": {},
		"🌍 Open Source":            {},
	}

	// Categorize authors
	for _, info := range authorInfos {
		category := categorizeAuthor(info.author)
		if list, ok := categories[category]; ok {
			categories[category] = append(list, info)
		}
	}

	// Display each category
	for _, category := range []string{
		"🏢 Major Tech Companies",
		"🚀 AI Startups",
		"🎓 Research Organizations",
		"🌍 Open Source",
	} {
		if authors := categories[category]; len(authors) > 0 {
			fmt.Fprintf(f, "### %s\n\n", category)

			for _, info := range authors {
				author := info.author
				desc := ""
				if author.Description != nil && *author.Description != "" {
					desc = *author.Description
					if len(desc) > 80 {
						desc = desc[:77] + "..."
					}
				}

				website := ""
				if author.Website != nil && *author.Website != "" {
					website = fmt.Sprintf(" | [Website](%s)", *author.Website)
				}

				fmt.Fprintf(f, "- **[%s](%s/)** - %d models - %s%s\n",
					author.Name, string(author.ID), info.modelCount, desc, website)
			}
			fmt.Fprintln(f)
		}
	}

	// Footer
	g.writeFooter(f, Breadcrumb{Label: "Back to Catalog", Path: "../"})

	return nil
}

// generateAuthorReadme generates documentation for a single author
func (g *Generator) generateAuthorReadme(dir string, author *catalogs.Author, catalog catalogs.Reader) error {
	readmeFile := filepath.Join(dir, "README.md")
	f, err := os.Create(readmeFile)
	if err != nil {
		return fmt.Errorf("creating README: %w", err)
	}
	defer f.Close()

	// Header with logo if available
	logoPath := fmt.Sprintf("https://raw.githubusercontent.com/agentstation/starmap/master/internal/embedded/logos/%s.svg", author.ID)
	fmt.Fprintf(f, "# <img src=\"%s\" alt=\"%s\" width=\"32\" height=\"32\" style=\"vertical-align: middle;\"> %s\n\n",
		logoPath, author.Name, author.Name)

	// Author description
	if author.Description != nil && *author.Description != "" {
		fmt.Fprintf(f, "%s\n\n", *author.Description)
	}

	// Author Information
	fmt.Fprintln(f, "## Organization Information")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Field | Value |")
	fmt.Fprintln(f, "|-------|-------|")
	fmt.Fprintf(f, "| **Author ID** | `%s` |\n", author.ID)
	fmt.Fprintf(f, "| **Type** | %s |\n", categorizeAuthor(author))

	if author.Website != nil && *author.Website != "" {
		fmt.Fprintf(f, "| **Website** | [%s](%s) |\n", *author.Website, *author.Website)
	}

	// Find all models by this author
	var authorModels []*catalogs.Model
	allModels := catalog.Models().List()
	providers := catalog.Providers().List()
	providerMap := make(map[string]int)

	for _, model := range allModels {
		for _, ma := range model.Authors {
			if ma.ID == author.ID {
				authorModels = append(authorModels, model)
				// Count providers that have this model
				for _, provider := range providers {
					if provider.Models != nil {
						if _, hasModel := provider.Models[model.ID]; hasModel {
							providerMap[string(provider.ID)]++
						}
					}
				}
				break
			}
		}
	}

	fmt.Fprintf(f, "| **Total Models** | %d |\n", len(authorModels))
	if len(providerMap) > 0 {
		fmt.Fprintf(f, "| **Available On** | %d providers |\n", len(providerMap))
	}

	fmt.Fprintln(f)

	// Models section
	fmt.Fprintln(f, "## Models")
	fmt.Fprintln(f)

	if len(authorModels) == 0 {
		fmt.Fprintln(f, "*No models found from this author.*")
	} else {
		// Group models by family
		modelFamilies := groupAuthorModels(authorModels)

		for family, models := range modelFamilies {
			if len(modelFamilies) > 1 && family != "" {
				fmt.Fprintf(f, "### %s\n\n", family)
			}

			fmt.Fprintln(f, "| Model | Providers | Context | Capabilities |")
			fmt.Fprintln(f, "|-------|-----------|---------|--------------|")

			// Sort models
			sort.Slice(models, func(i, j int) bool {
				return models[i].Name < models[j].Name
			})

			for _, model := range models {
				// Model link to local models subdirectory
				modelLink := fmt.Sprintf("[%s](./models/%s.md)", model.Name, formatModelID(model.ID))

				// Count providers that have this model
				providerCount := 0
				var firstProvider string
				for _, provider := range catalog.Providers().List() {
					if provider.Models != nil {
						if _, hasModel := provider.Models[model.ID]; hasModel {
							if providerCount == 0 {
								firstProvider = string(provider.ID)
							}
							providerCount++
						}
					}
				}
				providerStr := fmt.Sprintf("%d", providerCount)
				if providerCount == 1 {
					providerStr = firstProvider
				}

				// Context
				contextStr := "N/A"
				if model.Limits != nil && model.Limits.ContextWindow > 0 {
					contextStr = formatContext(model.Limits.ContextWindow)
				}

				// Capabilities from Features
				caps := []string{}
				if model.Features != nil {
					if hasText(model.Features) {
						caps = append(caps, "Text")
					}
					if hasVision(model.Features) {
						caps = append(caps, "Vision")
					}
					if hasAudio(model.Features) {
						caps = append(caps, "Audio")
					}
					if hasVideo(model.Features) {
						caps = append(caps, "Video")
					}
					if hasToolSupport(model.Features) {
						caps = append(caps, "Functions")
					}
				}
				capsStr := "—"
				if len(caps) > 0 {
					capsStr = ""
					for i, c := range caps {
						if i > 0 {
							capsStr += ", "
						}
						capsStr += c
					}
				}

				fmt.Fprintf(f, "| %s | %s | %s | %s |\n",
					modelLink, providerStr, contextStr, capsStr)
			}

			fmt.Fprintln(f)
		}

		// Provider availability
		if len(providerMap) > 0 {
			fmt.Fprintln(f, "## Provider Availability")
			fmt.Fprintln(f)
			fmt.Fprintln(f, "Models from this author are available through the following providers:")
			fmt.Fprintln(f)

			// Sort providers by model count
			type providerCount struct {
				name  string
				count int
			}
			var providers []providerCount
			for name, count := range providerMap {
				providers = append(providers, providerCount{name, count})
			}
			sort.Slice(providers, func(i, j int) bool {
				return providers[i].count > providers[j].count
			})

			for _, p := range providers {
				plural := "model"
				if p.count > 1 {
					plural = "models"
				}
				fmt.Fprintf(f, "- **[%s](../../providers/%s/)** - %d %s\n",
					p.name, p.name, p.count, plural)
			}
			fmt.Fprintln(f)
		}
	}

	// Research & Development section (if applicable)
	if shouldShowResearch(author) {
		fmt.Fprintln(f, "## Research & Development")
		fmt.Fprintln(f)
		fmt.Fprintln(f, getResearchInfo(author))
		fmt.Fprintln(f)
	}

	// Add cross-reference navigation
	crossRefs := g.buildAuthorCrossReferences(string(author.ID))
	if len(crossRefs) > 0 {
		g.writeNavigationSection(f, "See Also", crossRefs)
		fmt.Fprintln(f)
	}

	// Footer
	g.writeFooter(f,
		Breadcrumb{Label: "Back to Authors", Path: "../"},
		Breadcrumb{Label: "Back to Catalog", Path: "../../"})

	return nil
}

// generateAuthorModelPages generates individual model pages for an author
func (g *Generator) generateAuthorModelPages(dir string, author *catalogs.Author, catalog catalogs.Reader) error {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating models directory: %w", err)
	}
	
	// Find all models by this author
	allModels := catalog.Models().List()
	var authorModels []*catalogs.Model
	
	for _, model := range allModels {
		for _, ma := range model.Authors {
			if ma.ID == author.ID {
				authorModels = append(authorModels, model)
				break
			}
		}
	}
	
	// Generate a page for each model
	for _, model := range authorModels {
		if err := g.generateAuthorModelPage(dir, author, model, catalog); err != nil {
			return fmt.Errorf("generating model %s page: %w", model.ID, err)
		}
	}
	
	return nil
}

// generateAuthorModelPage generates a single model page in author context
func (g *Generator) generateAuthorModelPage(dir string, author *catalogs.Author, model *catalogs.Model, catalog catalogs.Reader) error {
	// Use getModelFilePath to preserve subdirectory structure
	modelFile, err := getModelFilePath(dir, model.ID)
	if err != nil {
		return fmt.Errorf("getting file path for model %s: %w", model.ID, err)
	}
	f, err := os.Create(modelFile)
	if err != nil {
		return fmt.Errorf("creating model file: %w", err)
	}
	defer f.Close()
	
	// Header with breadcrumb navigation
	fmt.Fprintln(f, "---")
	fmt.Fprintf(f, "title: \"%s\"\n", model.Name)
	fmt.Fprintf(f, "author: \"%s\"\n", author.Name)
	fmt.Fprintln(f, "type: model")
	fmt.Fprintln(f, "---")
	fmt.Fprintln(f)
	
	// Breadcrumb navigation
	breadcrumbs := g.authorModelBreadcrumb(author.Name, model.Name, model.ID)
	g.writeBreadcrumbs(f, breadcrumbs...)
	
	// Model header
	fmt.Fprintf(f, "# %s\n\n", model.Name)
	
	// Description
	if model.Description != "" {
		fmt.Fprintf(f, "%s\n\n", model.Description)
	}
	
	// Author attribution
	fmt.Fprintf(f, "**Developed by**: [%s](../)\n\n", author.Name)
	
	// Technical Specifications
	fmt.Fprintln(f, "## 📋 Technical Specifications")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Specification | Value |")
	fmt.Fprintln(f, "|--------------|-------|")
	fmt.Fprintf(f, "| **Model ID** | `%s` |\n", model.ID)
	
	// Context and limits
	if model.Limits != nil {
		if model.Limits.ContextWindow > 0 {
			fmt.Fprintf(f, "| **Context Window** | %s tokens |\n", formatNumber(int(model.Limits.ContextWindow)))
		}
		if model.Limits.OutputTokens > 0 {
			fmt.Fprintf(f, "| **Max Output** | %s tokens |\n", formatNumber(int(model.Limits.OutputTokens)))
		}
	}
	
	// Architecture
	if model.Metadata != nil {
		if model.Metadata.Architecture != nil && model.Metadata.Architecture.ParameterCount != "" {
			fmt.Fprintf(f, "| **Parameters** | %s |\n", model.Metadata.Architecture.ParameterCount)
		}
		if !model.Metadata.ReleaseDate.IsZero() {
			fmt.Fprintf(f, "| **Release Date** | %s |\n", model.Metadata.ReleaseDate.Format("2006-01-02"))
		}
		fmt.Fprintf(f, "| **Open Weights** | %t |\n", model.Metadata.OpenWeights)
	}
	
	fmt.Fprintln(f)
	
	// Capabilities
	fmt.Fprintln(f, "## 🎯 Capabilities")
	fmt.Fprintln(f)
	
	// Feature badges
	if model.Features != nil {
		badges := featureBadges(model)
		if badges != "" {
			fmt.Fprintln(f, badges)
			fmt.Fprintln(f)
		}
	}
	
	// Provider Availability - showing variations
	fmt.Fprintln(f, "## 🌐 Provider Availability")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "This model is available through the following providers with potential variations:")
	fmt.Fprintln(f)
	
	fmt.Fprintln(f, "| Provider | Context | Pricing (Input/Output) | Notes |")
	fmt.Fprintln(f, "|----------|---------|------------------------|-------|")
	
	providers := catalog.Providers().List()
	providerCount := 0
	for _, provider := range providers {
		if provider.Models != nil {
			if providerModel, hasModel := provider.Models[model.ID]; hasModel {
				providerCount++
				
				// Context (may vary by provider)
				contextStr := "—"
				if providerModel.Limits != nil && providerModel.Limits.ContextWindow > 0 {
					contextStr = formatContext(providerModel.Limits.ContextWindow)
				} else if model.Limits != nil && model.Limits.ContextWindow > 0 {
					contextStr = formatContext(model.Limits.ContextWindow)
				}
				
				// Pricing (provider-specific)
				pricingStr := "—"
				if providerModel.Pricing != nil && providerModel.Pricing.Tokens != nil {
					if providerModel.Pricing.Tokens.Input != nil && providerModel.Pricing.Tokens.Output != nil {
						pricingStr = fmt.Sprintf("$%.2f / $%.2f",
							providerModel.Pricing.Tokens.Input.Per1M,
							providerModel.Pricing.Tokens.Output.Per1M)
					}
				}
				
				// Notes about variations
				notes := ""
				if providerModel.Limits != nil && model.Limits != nil {
					if providerModel.Limits.ContextWindow != model.Limits.ContextWindow {
						notes = "Custom context"
					}
				}
				
				// Link to provider-specific page
				providerLink := fmt.Sprintf("[%s](../../../providers/%s/models/%s.md)",
					provider.Name, string(provider.ID), formatModelID(model.ID))
				
				fmt.Fprintf(f, "| %s | %s | %s | %s |\n",
					providerLink, contextStr, pricingStr, notes)
			}
		}
	}
	
	if providerCount == 0 {
		fmt.Fprintln(f, "| *No providers found* | | | |")
	}
	
	fmt.Fprintln(f)
	
	// Build navigation links
	navLinks := []NavigationLink{}
	navLinks = append(navLinks, NavigationLink{
		Label: fmt.Sprintf("All %s Models", author.Name),
		Path:  "../",
	})
	
	// Calculate depth for proper paths
	depth := 4
	if strings.Contains(model.ID, "/") {
		depth += strings.Count(model.ID, "/")
	}
	
	// Add cross-references using path helpers
	modelsPath := getModelsPath(depth)
	navLinks = append(navLinks, NavigationLink{
		Label: "Model Comparison",
		Path:  modelsPath + "/",
	})
	
	// Write navigation section
	g.writeNavigationSection(f, "🔗 Related Resources", navLinks)
	
	// Other models by same author
	var otherModels []string
	for _, m := range catalog.Models().List() {
		if m.ID != model.ID {
			for _, ma := range m.Authors {
				if ma.ID == author.ID {
					otherModels = append(otherModels, m.Name)
					break
				}
			}
		}
	}
	
	if len(otherModels) > 0 {
		fmt.Fprintln(f)
		fmt.Fprintln(f, "### Other Models by This Author")
		fmt.Fprintln(f)
		for _, name := range otherModels {
			if len(otherModels) > 5 {
				// Just list first 5
				if len(otherModels) > 5 {
					fmt.Fprintln(f, "- ", name)
				}
			} else {
				fmt.Fprintln(f, "- ", name)
			}
		}
		if len(otherModels) > 5 {
			fmt.Fprintf(f, "- _...and %d more_\n", len(otherModels)-5)
		}
	}
	
	fmt.Fprintln(f)
	
	// Footer with timestamp
	g.writeTimestampedFooter(f)
	
	return nil
}