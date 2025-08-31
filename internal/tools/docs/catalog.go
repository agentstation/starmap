package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	md "github.com/nao1215/markdown"
)

// generateCatalogIndex generates the main catalog index page
func (g *Generator) generateCatalogIndex(dir string, catalog catalogs.Reader) error {
	// Write to both README.md (for GitHub) and _index.md (for Hugo)
	for _, filename := range []string{"README.md", "_index.md"} {
		indexFile := filepath.Join(dir, filename)
		f, err := os.Create(indexFile)
		if err != nil {
			return fmt.Errorf("creating index file %s: %w", filename, err)
		}
		if err := g.writeCatalogContent(f, catalog); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

// writeCatalogContent writes the catalog content to the given writer
func (g *Generator) writeCatalogContent(f *os.File, catalog catalogs.Reader) error {

	builder := NewMarkdownBuilder(f)
	
	// Generate header
	builder.H1("🌟 Starmap AI Model Catalog").LF()
	builder.PlainText("A comprehensive catalog of AI models from various providers, with detailed specifications, pricing, and capabilities.").LF().LF()

	// Add last updated
	builder.Italic(fmt.Sprintf("_Last Updated: %s_", time.Now().Format("January 2, 2006"))).LF().LF()

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
	builder.H2("📊 Catalog Statistics").LF()
	
	statsRows := [][]string{
		{"**Total Models**", formatNumber(len(models))},
		{"**Providers**", fmt.Sprintf("%d", len(providers))},
		{"**Model Authors**", fmt.Sprintf("%d", len(authors))},
		{"**Models with Pricing**", fmt.Sprintf("%d (%.1f%%)", modelsWithPricing, float64(modelsWithPricing)/float64(len(models))*100)},
	}
	if len(models) > 0 {
		statsRows = append(statsRows, []string{
			"**Average Context**",
			fmt.Sprintf("%s tokens", formatNumber(int(totalContextWindow/int64(len(models))))),
		})
	}
	
	builder.Table(md.TableSet{
		Header: []string{"Metric", "Value"},
		Rows:   statsRows,
	}).LF()

	// Quick Navigation
	builder.H2("🚀 Quick Navigation").LF()
	builder.H3("Browse by Provider").LF()

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

	providerRows := [][]string{}
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
		providerRows = append(providerRows, []string{
			fmt.Sprintf("%s [%s](providers/%s/)", badge, provider.Name, string(provider.ID)),
			fmt.Sprintf("%d", pi.modelCount),
			latestModel,
		})
	}
	
	builder.Table(md.TableSet{
		Header: []string{"Provider", "Models", "Latest Addition"},
		Rows:   providerRows,
	}).LF()

	// Browse by Author
	builder.H3("Browse by Model Author").LF()

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

	authorRows := [][]string{}
	displayCount := min(10, len(authorInfos))
	for _, ai := range authorInfos[:displayCount] {
		author := ai.author
		desc := "AI research organization"
		if author.Description != nil && *author.Description != "" {
			desc = *author.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
		}

		badge := getAuthorBadge(author.Name)
		authorRows = append(authorRows, []string{
			fmt.Sprintf("%s [%s](authors/%s/)", badge, author.Name, string(author.ID)),
			fmt.Sprintf("%d", ai.modelCount),
			desc,
		})
	}
	
	builder.Table(md.TableSet{
		Header: []string{"Author", "Models", "Description"},
		Rows:   authorRows,
	})

	if len(authorInfos) > 10 {
		builder.LF().PlainTextf("[View all %d authors →](authors/)", len(authorInfos))
	}
	
	builder.LF().LF()

	// Featured Models section
	builder.H2("⭐ Featured Models").LF()
	builder.H3("Latest & Greatest").LF()

	// Show a selection of notable models
	featuredModels := selectFeaturedModels(models)
	
	featuredRows := [][]string{}
	featuredCount := min(10, len(featuredModels))
	for _, model := range featuredModels[:featuredCount] {
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

		featuredRows = append(featuredRows, []string{
			fmt.Sprintf("**[%s](models/%s.md)**", model.Name, formatModelID(model.ID)),
			providerName,
			contextStr,
			pricingStr,
		})
	}

	builder.Table(md.TableSet{
		Header: []string{"Model", "Provider", "Context", "Pricing"},
		Rows:   featuredRows,
	}).LF()

	// Browse Options
	builder.H2("📚 Browse Options").LF()
	builder.BulletList(
		"🏢 **[By Provider](providers/)** - Browse models grouped by their hosting provider",
		"👥 **[By Author](authors/)** - Browse models grouped by their creating organization",
		"📋 **[All Models](models/)** - Complete alphabetical listing of all models",
	).LF()

	// Footer (catalog root doesn't have back links)
	builder.Build()
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