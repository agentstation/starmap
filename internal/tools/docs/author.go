package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	md "github.com/nao1215/markdown"
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
	
	// Write to both README.md (for GitHub) and _index.md (for Hugo)
	for _, filename := range []string{"README.md", "_index.md"} {
		indexFile := filepath.Join(dir, filename)
		f, err := os.Create(indexFile)
		if err != nil {
			return fmt.Errorf("creating author index %s: %w", filename, err)
		}
		if err := g.writeAuthorIndexContent(f, authors, models, catalog); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

// writeAuthorIndexContent writes the author index content to the given writer
func (g *Generator) writeAuthorIndexContent(f *os.File, authors []*catalogs.Author, models []*catalogs.Model, catalog catalogs.Reader) error {

	builder := NewMarkdownBuilder(f)
	builder.H1("👥 Model Authors").LF()
	builder.PlainText("Organizations and researchers that develop and train AI models.").LF().LF()

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
	builder.H2("Author Overview").LF()

	headers := []string{"Author", "Models", "Hosted By", "Focus Area"}
	var rows [][]string

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
				// Build provider count string
				providerBuilder := NewMarkdownBuilderBuffer()
				providerBuilder.CountText(len(providerList), "provider", "providers")
				providerBuilder.Build()
				providersStr = providerBuilder.String()
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

		// Build author name with badge and link
		authorCell := NewMarkdownBuilderBuffer()
		authorCell.PlainText(badge).PlainText(" ").BoldLink(author.Name, string(author.ID)+"/")
		authorCell.Build()
		
		rows = append(rows, []string{
			authorCell.String(),
			fmt.Sprintf("%d", info.modelCount),
			providersStr,
			focusArea,
		})
	}

	builder.Table(md.TableSet{
		Header: headers,
		Rows:   rows,
	}).LF()

	// Categories section
	builder.H2("By Category").LF()

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
			builder.H3(category).LF()

			var items []string
			for _, info := range authors {
				author := info.author
				desc := ""
				if author.Description != nil && *author.Description != "" {
					desc = *author.Description
					if len(desc) > 80 {
						// Truncate description
				descBuilder := NewMarkdownBuilderBuffer()
				descBuilder.TruncateText(desc, 80)
				descBuilder.Build()
				desc = descBuilder.String()
					}
				}

				// Build list item using builder
				itemBuilder := NewMarkdownBuilderBuffer()
				itemBuilder.BoldLink(author.Name, string(author.ID)+"/").
					PlainText(" - ").
					CountText(info.modelCount, "model", "models").
					PlainText(" - ").
					PlainText(desc)
				
				if author.Website != nil && *author.Website != "" {
					itemBuilder.PlainText(" | ").Link("Website", *author.Website)
				}
				
				itemBuilder.Build()
				item := itemBuilder.String()
				items = append(items, item)
			}
			builder.BulletList(items...).LF()
		}
	}

	// Footer
	builder.HorizontalRule()
	builder.Italic(g.buildFooter(Breadcrumb{Label: "Back to Catalog", Path: "../"}))

	return builder.Build()
}

// generateAuthorReadme generates documentation for a single author
func (g *Generator) generateAuthorReadme(dir string, author *catalogs.Author, catalog catalogs.Reader) error {
	// Write to both README.md (for GitHub) and _index.md (for Hugo)
	for _, filename := range []string{"README.md", "_index.md"} {
		readmeFile := filepath.Join(dir, filename)
		f, err := os.Create(readmeFile)
		if err != nil {
			return fmt.Errorf("creating %s: %w", filename, err)
		}
		if err := g.writeAuthorReadmeContent(f, author, catalog); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	
	return nil
}

// writeAuthorReadmeContent writes the author readme content to the given writer
func (g *Generator) writeAuthorReadmeContent(f *os.File, author *catalogs.Author, catalog catalogs.Reader) error {

	builder := NewMarkdownBuilder(f)

	// Header with logo if available
	// Build title with logo using builder
	logoPath := "https://raw.githubusercontent.com/agentstation/starmap/master/internal/embedded/logos/" + string(author.ID) + ".svg"
	titleBuilder := NewMarkdownBuilderBuffer()
	titleBuilder.PlainTextf("# <img src=\"%s\" alt=\"%s logo\" width=\"48\" height=\"48\" style=\"vertical-align: middle;\"> %s", logoPath, author.Name, author.Name)
	titleBuilder.Build()
	builder.RawHTML(titleBuilder.String()).LF().LF()

	// Author description
	if author.Description != nil && *author.Description != "" {
		builder.PlainText(*author.Description).LF().LF()
	}

	// Author Information
	builder.H2("Organization Information").LF()

	infoHeaders := []string{"Field", "Value"}
	infoRows := [][]string{
		{"**Author ID**", "`" + string(author.ID) + "`"},
		{"**Type**", categorizeAuthor(author)},
	}

	if author.Website != nil && *author.Website != "" {
		// Build website link using builder
		websiteBuilder := NewMarkdownBuilderBuffer()
		websiteBuilder.Link(*author.Website, *author.Website)
		websiteBuilder.Build()
		infoRows = append(infoRows, []string{"**Website**", websiteBuilder.String()})
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

	// Build count strings using builder
	modelCountBuilder := NewMarkdownBuilderBuffer()
	modelCountBuilder.PlainTextf("%d", len(authorModels))
	modelCountBuilder.Build()
	infoRows = append(infoRows, []string{"**Total Models**", modelCountBuilder.String()})
	
	if len(providerMap) > 0 {
		providerCountBuilder := NewMarkdownBuilderBuffer()
		providerCountBuilder.CountText(len(providerMap), "provider", "providers")
		providerCountBuilder.Build()
		infoRows = append(infoRows, []string{"**Available On**", providerCountBuilder.String()})
	}

	builder.Table(md.TableSet{
		Header: infoHeaders,
		Rows:   infoRows,
	}).LF()

	// Models section
	builder.H2("Models").LF()

	if len(authorModels) == 0 {
		builder.Italic("No models found from this author.").LF()
	} else {
		// Group models by family
		modelFamilies := groupAuthorModels(authorModels)

		for family, models := range modelFamilies {
			if len(modelFamilies) > 1 && family != "" {
				builder.H3(family).LF()
			}

			modelHeaders := []string{"Model", "Providers", "Context", "Capabilities"}
			var modelRows [][]string

			// Sort models (make a copy to avoid modifying the original)
			modelsCopy := make([]*catalogs.Model, len(models))
			copy(modelsCopy, models)
			sort.Slice(modelsCopy, func(i, j int) bool {
				return modelsCopy[i].Name < modelsCopy[j].Name
			})

			for _, model := range modelsCopy {
				// Model link to local models subdirectory
				modelLink := fmt.Sprintf("[%s](./models/%s)", model.Name, formatModelID(model.ID))

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

				modelRows = append(modelRows, []string{modelLink, providerStr, contextStr, capsStr})
			}

			builder.Table(md.TableSet{
				Header: modelHeaders,
				Rows:   modelRows,
			}).LF()
		}

		// Provider availability
		if len(providerMap) > 0 {
			builder.H2("Provider Availability").LF()
			builder.PlainText("Models from this author are available through the following providers:").LF().LF()

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

			var providerItems []string
			for _, p := range providers {
				plural := "model"
				if p.count > 1 {
					plural = "models"
				}
				item := fmt.Sprintf("**[%s](../../providers/%s/)** - %d %s", p.name, p.name, p.count, plural)
				providerItems = append(providerItems, item)
			}
			builder.BulletList(providerItems...).LF()
		}
	}

	// Research & Development section (if applicable)
	if shouldShowResearch(author) {
		builder.H2("Research & Development").LF()
		builder.PlainText(getResearchInfo(author)).LF()
	}

	// Add cross-reference navigation
	crossRefs := g.buildAuthorCrossReferences(string(author.ID))
	if len(crossRefs) > 0 {
		builder.H3("See Also").LF()
		var items []string
		for _, link := range crossRefs {
			if link.Description != "" {
				item := fmt.Sprintf("[%s](%s) - %s", link.Label, link.Path, link.Description)
				items = append(items, item)
			} else {
				item := fmt.Sprintf("[%s](%s)", link.Label, link.Path)
				items = append(items, item)
			}
		}
		builder.BulletList(items...).LF()
	}

	// Footer
	builder.HorizontalRule()
	builder.Italic(g.buildFooter(
		Breadcrumb{Label: "Back to Authors", Path: "../"},
		Breadcrumb{Label: "Back to Catalog", Path: "../../"}))

	return builder.Build()
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
	
	builder := NewMarkdownBuilder(f)
	
	// Header with metadata
	builder.RawHTML("---").LF()
	builder.PlainTextf("title: \"%s\"", model.Name).LF()
	builder.PlainTextf("author: \"%s\"", author.Name).LF()
	builder.PlainText("type: model").LF()
	builder.RawHTML("---").LF().LF()
	
	// Breadcrumb navigation
	breadcrumbs := g.authorModelBreadcrumb(author.Name, model.Name, model.ID)
	breadcrumbStr := g.buildBreadcrumbs(breadcrumbs...)
	if breadcrumbStr != "" {
		builder.PlainText(breadcrumbStr).LF().LF()
	}
	
	// Model header
	builder.H1(model.Name).LF()
	
	// Description
	if model.Description != "" {
		builder.PlainText(model.Description).LF().LF()
	}
	
	// Author attribution
	builder.Bold("Developed by").PlainText(": ").Link(author.Name, "../").LF().LF()
	
	// Technical Specifications
	builder.H2("📋 Technical Specifications").LF()
	
	specHeaders := []string{"Specification", "Value"}
	specRows := [][]string{
		{"**Model ID**", fmt.Sprintf("`%s`", model.ID)},
	}
	
	// Context and limits
	if model.Limits != nil {
		if model.Limits.ContextWindow > 0 {
			specRows = append(specRows, []string{"**Context Window**", fmt.Sprintf("%s tokens", formatNumber(int(model.Limits.ContextWindow)))})
		}
		if model.Limits.OutputTokens > 0 {
			specRows = append(specRows, []string{"**Max Output**", fmt.Sprintf("%s tokens", formatNumber(int(model.Limits.OutputTokens)))})
		}
	}
	
	// Architecture
	if model.Metadata != nil {
		if model.Metadata.Architecture != nil && model.Metadata.Architecture.ParameterCount != "" {
			specRows = append(specRows, []string{"**Parameters**", model.Metadata.Architecture.ParameterCount})
		}
		if !model.Metadata.ReleaseDate.IsZero() {
			specRows = append(specRows, []string{"**Release Date**", model.Metadata.ReleaseDate.Format("2006-01-02")})
		}
		specRows = append(specRows, []string{"**Open Weights**", fmt.Sprintf("%t", model.Metadata.OpenWeights)})
	}
	
	builder.Table(md.TableSet{
		Header: specHeaders,
		Rows:   specRows,
	}).LF()
	
	// Capabilities
	builder.H2("🎯 Capabilities").LF()
	
	// Feature badges
	if model.Features != nil {
		badges := featureBadges(model)
		if badges != "" {
			builder.PlainText(badges).LF().LF()
		}
	}
	
	// Provider Availability - showing variations
	builder.H2("🌐 Provider Availability").LF()
	builder.PlainText("This model is available through the following providers with potential variations:").LF().LF()
	
	providerHeaders := []string{"Provider", "Context", "Pricing (Input/Output)", "Notes"}
	var providerRows [][]string
	
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
				providerLink := fmt.Sprintf("[%s](../../../providers/%s/models/%s)",
					provider.Name, string(provider.ID), formatModelID(model.ID))
				
				providerRows = append(providerRows, []string{providerLink, contextStr, pricingStr, notes})
			}
		}
	}
	
	if providerCount == 0 {
		providerRows = append(providerRows, []string{"*No providers found*", "", "", ""})
	}
	
	builder.Table(md.TableSet{
		Header: providerHeaders,
		Rows:   providerRows,
	}).LF()
	
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
	builder.H3("🔗 Related Resources").LF()
	var navItems []string
	for _, link := range navLinks {
		if link.Description != "" {
			item := fmt.Sprintf("[%s](%s) - %s", link.Label, link.Path, link.Description)
			navItems = append(navItems, item)
		} else {
			item := fmt.Sprintf("[%s](%s)", link.Label, link.Path)
			navItems = append(navItems, item)
		}
	}
	builder.BulletList(navItems...).LF()
	
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
		builder.LF().H3("Other Models by This Author").LF()
		
		displayModels := otherModels
		if len(otherModels) > 5 {
			displayModels = otherModels[:5]
		}
		
		builder.BulletList(displayModels...)
		
		if len(otherModels) > 5 {
			builder.PlainTextf("- _...and %d more_", len(otherModels)-5).LF()
		}
	}
	
	builder.LF()
	
	// Footer with timestamp
	builder.HorizontalRule()
	builder.Italic(fmt.Sprintf("Last Updated: %s | %s",
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		"Generated by [Starmap](https://github.com/agentstation/starmap)"))
	
	return builder.Build()
}

// Helper functions for builder-based operations

// writeFooterToBuilder writes a footer using the markdown builder
func (g *Generator) writeFooterToBuilder(builder *MarkdownBuilder, breadcrumbs ...Breadcrumb) {
	var buf strings.Builder
	g.writeFooter(&buf, breadcrumbs...)
	builder.PlainText(buf.String())
}

// writeNavigationSectionToBuilder writes a navigation section using the markdown builder
func (g *Generator) writeNavigationSectionToBuilder(builder *MarkdownBuilder, title string, links []NavigationLink) {
	var buf strings.Builder
	g.writeNavigationSection(&buf, title, links)
	builder.PlainText(buf.String())
}

// writeTimestampedFooterToBuilder writes a timestamped footer using the markdown builder
func (g *Generator) writeTimestampedFooterToBuilder(builder *MarkdownBuilder) {
	var buf strings.Builder
	g.writeTimestampedFooter(&buf)
	builder.PlainText(buf.String())
}

// writeBreadcrumbsToBuilder writes breadcrumbs using the markdown builder
func (g *Generator) writeBreadcrumbsToBuilder(builder *MarkdownBuilder, breadcrumbs ...Breadcrumb) {
	var buf strings.Builder
	g.writeBreadcrumbs(&buf, breadcrumbs...)
	builder.PlainText(buf.String())
}