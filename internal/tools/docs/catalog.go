package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// generateCatalogIndex generates the main catalog index page
func (g *Generator) generateCatalogIndex(dir string, catalog catalogs.Reader) error {
	indexFile := filepath.Join(dir, "README.md")
	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("creating index file: %w", err)
	}
	defer f.Close()

	// Generate header
	fmt.Fprintln(f, "# 🌟 Starmap AI Model Catalog")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "A comprehensive catalog of AI models from various providers, with detailed specifications, pricing, and capabilities.")
	fmt.Fprintln(f)

	// Add last updated
	fmt.Fprintf(f, "_Last Updated: %s_\n\n", time.Now().Format("January 2, 2006"))

	// Count statistics
	providers := catalog.Providers().List()
	authors := catalog.Authors().List()
	models := catalog.Models().List()

	// Calculate aggregate stats
	totalContextWindow := int64(0)
	modelsWithPricing := 0
	for _, model := range models {
		if model.Limits != nil && model.Limits.ContextWindow > 0 {
			totalContextWindow += model.Limits.ContextWindow
		}
		if model.Pricing != nil && model.Pricing.Tokens != nil {
			modelsWithPricing++
		}
	}

	// Statistics section
	fmt.Fprintln(f, "## 📊 Catalog Statistics")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Metric | Value |")
	fmt.Fprintln(f, "|--------|-------|")
	fmt.Fprintf(f, "| **Total Models** | %s |\n", formatNumber(len(models)))
	fmt.Fprintf(f, "| **Providers** | %d |\n", len(providers))
	fmt.Fprintf(f, "| **Model Authors** | %d |\n", len(authors))
	fmt.Fprintf(f, "| **Models with Pricing** | %d (%.1f%%) |\n",
		modelsWithPricing, float64(modelsWithPricing)/float64(len(models))*100)
	if len(models) > 0 {
		fmt.Fprintf(f, "| **Average Context** | %s tokens |\n",
			formatNumber(int(totalContextWindow/int64(len(models)))))
	}
	fmt.Fprintln(f)

	// Quick Navigation
	fmt.Fprintln(f, "## 🚀 Quick Navigation")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "### Browse by Provider")
	fmt.Fprintln(f)

	// Sort providers by model count
	type providerInfo struct {
		provider   *catalogs.Provider
		modelCount int
	}
	var providerInfos []providerInfo
	for _, provider := range providers {
		providerInfos = append(providerInfos, providerInfo{
			provider:   provider,
			modelCount: len(provider.Models),
		})
	}
	sort.Slice(providerInfos, func(i, j int) bool {
		return providerInfos[i].modelCount > providerInfos[j].modelCount
	})

	fmt.Fprintln(f, "| Provider | Models | Latest Addition |")
	fmt.Fprintln(f, "|----------|--------|-----------------|")

	for _, pi := range providerInfos {
		provider := pi.provider
		latestModel := "N/A"

		// Find latest model (simplified for now - just take the first one we find)
		if len(provider.Models) > 0 {
			for _, model := range provider.Models {
				latestModel = model.Name
				break // Just take the first one
			}
		}

		badge := getProviderBadge(provider.Name)
		fmt.Fprintf(f, "| %s [%s](providers/%s/) | %d | %s |\n",
			badge, provider.Name, string(provider.ID), pi.modelCount, latestModel)
	}

	fmt.Fprintln(f)

	// Browse by Author
	fmt.Fprintln(f, "### Browse by Model Author")
	fmt.Fprintln(f)

	// Sort authors by model count
	type authorInfo struct {
		author      *catalogs.Author
		modelCount  int
	}
	var authorInfos []authorInfo
	for _, author := range authors {
		count := 0
		for _, model := range models {
			for _, ma := range model.Authors {
				if ma.ID == author.ID {
					count++
					break
				}
			}
		}
		authorInfos = append(authorInfos, authorInfo{
			author:     author,
			modelCount: count,
		})
	}
	sort.Slice(authorInfos, func(i, j int) bool {
		return authorInfos[i].modelCount > authorInfos[j].modelCount
	})

	fmt.Fprintln(f, "| Author | Models | Description |")
	fmt.Fprintln(f, "|--------|--------|-------------|")

	for _, ai := range authorInfos[:min(10, len(authorInfos))] {
		author := ai.author
		desc := "AI research organization"
		if author.Description != nil && *author.Description != "" {
			desc = *author.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
		}

		badge := getAuthorBadge(author.Name)
		fmt.Fprintf(f, "| %s [%s](authors/%s/) | %d | %s |\n",
			badge, author.Name, string(author.ID), ai.modelCount, desc)
	}

	if len(authorInfos) > 10 {
		fmt.Fprintf(f, "\n[View all %d authors →](authors/)\n", len(authorInfos))
	}

	fmt.Fprintln(f)

	// Featured Models section
	fmt.Fprintln(f, "## ⭐ Featured Models")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "### Latest & Greatest")
	fmt.Fprintln(f)

	// Show a selection of notable models
	featuredModels := selectFeaturedModels(models)

	fmt.Fprintln(f, "| Model | Provider | Context | Pricing |")
	fmt.Fprintln(f, "|-------|----------|---------|---------|")

	for _, model := range featuredModels[:min(10, len(featuredModels))] {
		contextStr := "N/A"
		if model.Limits != nil && model.Limits.ContextWindow > 0 {
			contextStr = formatContext(model.Limits.ContextWindow)
		}

		pricingStr := "N/A"
		if model.Pricing != nil && model.Pricing.Tokens != nil && model.Pricing.Tokens.Input != nil {
			pricingStr = fmt.Sprintf("$%.2f/$%.2f",
				model.Pricing.Tokens.Input.Per1M,
				model.Pricing.Tokens.Output.Per1M)
		}

		// Find provider name
		providerName := "Unknown"
		for _, p := range providers {
			for _, pm := range p.Models {
				if pm.ID == model.ID {
					providerName = p.Name
					break
				}
			}
		}

		fmt.Fprintf(f, "| **[%s](models/%s.md)** | %s | %s | %s |\n",
			model.Name, formatModelID(model.ID), providerName, contextStr, pricingStr)
	}

	fmt.Fprintln(f)

	// Browse Options
	fmt.Fprintln(f, "## 📚 Browse Options")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "- 🏢 **[By Provider](providers/)** - Browse models grouped by their hosting provider")
	fmt.Fprintln(f, "- 👥 **[By Author](authors/)** - Browse models grouped by their creating organization")
	fmt.Fprintln(f, "- 📋 **[All Models](models/)** - Complete alphabetical listing of all models")
	fmt.Fprintln(f)

	// Footer (catalog root doesn't have back links)
	g.writeFooter(f)

	return nil
}

// selectFeaturedModels selects notable models to feature
func selectFeaturedModels(models []*catalogs.Model) []*catalogs.Model {
	// For now, just return models with pricing info, sorted by name
	var featured []*catalogs.Model
	for _, model := range models {
		if model.Pricing != nil && model.Pricing.Tokens != nil {
			featured = append(featured, model)
		}
	}

	sort.Slice(featured, func(i, j int) bool {
		// Prioritize popular model families
		iPriority := getModelPriority(featured[i].Name)
		jPriority := getModelPriority(featured[j].Name)
		if iPriority != jPriority {
			return iPriority < jPriority
		}
		return featured[i].Name < featured[j].Name
	})

	return featured
}

// getModelPriority returns a priority score for featuring models
func getModelPriority(name string) int {
	priorities := map[string]int{
		"GPT-4":    1,
		"Claude":   2,
		"Gemini":   3,
		"Llama":    4,
		"Mixtral":  5,
		"DeepSeek": 6,
	}

	for prefix, priority := range priorities {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return priority
		}
	}
	return 999
}