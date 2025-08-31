package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	md "github.com/nao1215/markdown"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// generateModelDocs generates documentation for all models
func (g *Generator) generateModelDocs(dir string, catalog catalogs.Reader) error {
	models := catalog.Models().List()
	providers := catalog.Providers().List()

	// Only generate the model comparison index
	// Individual model pages are now generated under providers and authors
	if err := g.generateModelIndex(dir, models, providers); err != nil {
		return fmt.Errorf("generating model index: %w", err)
	}

	return nil
}

// generateModelIndex generates the main model listing page
func (g *Generator) generateModelIndex(dir string, models []*catalogs.Model, providers []*catalogs.Provider) error {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}
	
	// Write to both README.md (for GitHub) and _index.md (for Hugo)
	for _, filename := range []string{"README.md", "_index.md"} {
		indexFile := filepath.Join(dir, filename)
		f, err := os.Create(indexFile)
		if err != nil {
			return fmt.Errorf("creating model index %s: %w", filename, err)
		}
		if err := g.writeModelIndexContent(f, models, providers); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	
	return nil
}

// writeModelIndexContent writes the model index content to the given writer
func (g *Generator) writeModelIndexContent(f *os.File, models []*catalogs.Model, providers []*catalogs.Provider) error {

	builder := NewMarkdownBuilder(f)
	
	builder.H1("🤖 All Models").
		LF().
		PlainTextf("Complete listing of all %d models in the Starmap catalog.", len(models)).
		LF().LF()

	// Quick stats
	builder.H2("Quick Stats").LF()

	// Count by capability
	textCount, visionCount, audioCount, functionCount := 0, 0, 0, 0
	for _, model := range models {
		if model.Features != nil {
			if hasText(model.Features) {
				textCount++
			}
			if hasVision(model.Features) {
				visionCount++
			}
			if hasAudio(model.Features) {
				audioCount++
			}
			if hasToolSupport(model.Features) {
				functionCount++
			}
		}
	}

	tableRows := [][]string{
		{"📝 Text Generation", fmt.Sprintf("%d", textCount), fmt.Sprintf("%.1f%%", float64(textCount)/float64(len(models))*100)},
		{"👁️ Vision", fmt.Sprintf("%d", visionCount), fmt.Sprintf("%.1f%%", float64(visionCount)/float64(len(models))*100)},
		{"🎵 Audio", fmt.Sprintf("%d", audioCount), fmt.Sprintf("%.1f%%", float64(audioCount)/float64(len(models))*100)},
		{"🔧 Function Calling", fmt.Sprintf("%d", functionCount), fmt.Sprintf("%.1f%%", float64(functionCount)/float64(len(models))*100)},
	}

	builder.Table(md.TableSet{
		Header: []string{"Capability", "Count", "Percentage"},
		Rows:   tableRows,
	}).LF()

	// Model families
	families := make(map[string][]*catalogs.Model)
	for _, model := range models {
		family := detectModelFamily(model.Name)
		families[family] = append(families[family], model)
	}

	// Sort families by size
	type familyInfo struct {
		name   string
		models []*catalogs.Model
	}
	var familyList []familyInfo
	for name, models := range families {
		familyList = append(familyList, familyInfo{name, models})
	}
	sort.Slice(familyList, func(i, j int) bool {
		return len(familyList[i].models) > len(familyList[j].models)
	})

	// Model families section
	builder.H2("Model Families").LF()

	for _, family := range familyList {
		if len(family.models) < 2 {
			continue // Skip families with only one model
		}

		builder.H3(fmt.Sprintf("%s (%d models)", family.name, len(family.models))).LF()

		// Sort models within family
		sort.Slice(family.models, func(i, j int) bool {
			return family.models[i].Name < family.models[j].Name
		})

		var familyTableRows [][]string
		displayCount := 0
		for _, model := range family.models {
			if displayCount >= 10 {
				familyTableRows = append(familyTableRows, []string{
					fmt.Sprintf("_...and %d more_", len(family.models)-10), "", "", ""})
				break
			}

			// Model link - point to author version if available, otherwise first provider
			modelLink := model.Name
			if len(model.Authors) > 0 {
				// Link to first author's version
				modelLink = fmt.Sprintf("[%s](../authors/%s/models/%s.md)", 
					model.Name, string(model.Authors[0].ID), formatModelID(string(model.ID)))
			}

			// Provider (would need to look this up from catalog)
			providerStr := "Multiple"

			// Context
			contextStr := "N/A"
			if model.Limits != nil && model.Limits.ContextWindow > 0 {
				contextStr = formatContext(model.Limits.ContextWindow)
			}

			// Pricing
			pricingStr := "N/A"
			if model.Pricing != nil && model.Pricing.Tokens != nil {
				if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Output != nil {
					pricingStr = fmt.Sprintf("$%.2f/$%.2f",
						model.Pricing.Tokens.Input.Per1M,
						model.Pricing.Tokens.Output.Per1M)
				}
			}

			familyTableRows = append(familyTableRows, []string{
				modelLink, providerStr, contextStr, pricingStr})

			displayCount++
		}

		builder.Table(md.TableSet{
			Header: []string{"Model", "Provider", "Context", "Pricing"},
			Rows:   familyTableRows,
		}).LF()
	}

	// Add Pricing Comparison section
	builder.H2("💰 Pricing Comparison").
		LF().
		PlainText("Top models by pricing (sorted by input cost):").
		LF()
	
	writePricingComparisonTable(f, models)
	builder.LF()

	// Add Context Limits Comparison section
	builder.H2("📏 Context Window Comparison").
		LF().
		PlainText("Top models by context window size:").
		LF()
	
	writeContextLimitsTable(f, models)
	builder.LF()

	// Add Feature Comparison section
	builder.H2("🎯 Feature Comparison").
		LF().
		PlainText("Detailed feature breakdown across models:").
		LF()
	
	writeFeatureComparisonTable(f, models)
	builder.LF()

	// Write content to file
	builder.Build()

	// Footer
	g.writeFooter(f, Breadcrumb{Label: "Back to Catalog", Path: "../"})

	return nil
}