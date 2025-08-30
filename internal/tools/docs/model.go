package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

// generateModelDocs generates documentation for all models
func (g *Generator) generateModelDocs(dir string, catalog catalogs.Reader) error {
	models := catalog.Models().List()

	// Only generate the model comparison index
	// Individual model pages are now generated under providers and authors
	if err := g.generateModelIndex(dir, models); err != nil {
		return fmt.Errorf("generating model index: %w", err)
	}

	return nil
}

// generateModelIndex generates the main model listing page
func (g *Generator) generateModelIndex(dir string, models []*catalogs.Model) error {
	// Ensure the directory exists
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}
	
	indexFile := filepath.Join(dir, "README.md")
	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("creating model index: %w", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "# 🤖 All Models")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "Complete listing of all %d models in the Starmap catalog.\n\n", len(models))

	// Quick stats
	fmt.Fprintln(f, "## Quick Stats")
	fmt.Fprintln(f)

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

	fmt.Fprintln(f, "| Capability | Count | Percentage |")
	fmt.Fprintln(f, "|------------|-------|------------|")
	fmt.Fprintf(f, "| 📝 Text Generation | %d | %.1f%% |\n", textCount, float64(textCount)/float64(len(models))*100)
	fmt.Fprintf(f, "| 👁️ Vision | %d | %.1f%% |\n", visionCount, float64(visionCount)/float64(len(models))*100)
	fmt.Fprintf(f, "| 🎵 Audio | %d | %.1f%% |\n", audioCount, float64(audioCount)/float64(len(models))*100)
	fmt.Fprintf(f, "| 🔧 Function Calling | %d | %.1f%% |\n", functionCount, float64(functionCount)/float64(len(models))*100)
	fmt.Fprintln(f)

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
	fmt.Fprintln(f, "## Model Families")
	fmt.Fprintln(f)

	for _, family := range familyList {
		if len(family.models) < 2 {
			continue // Skip families with only one model
		}

		fmt.Fprintf(f, "### %s (%d models)\n\n", family.name, len(family.models))

		fmt.Fprintln(f, "| Model | Provider | Context | Pricing |")
		fmt.Fprintln(f, "|-------|----------|---------|---------|")

		// Sort models within family
		sort.Slice(family.models, func(i, j int) bool {
			return family.models[i].Name < family.models[j].Name
		})

		displayCount := 0
		for _, model := range family.models {
			if displayCount >= 10 {
				fmt.Fprintf(f, "| _...and %d more_ | | | |\n", len(family.models)-10)
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

			fmt.Fprintf(f, "| %s | %s | %s | %s |\n",
				modelLink, providerStr, contextStr, pricingStr)

			displayCount++
		}

		fmt.Fprintln(f)
	}

	// Add Pricing Comparison section
	fmt.Fprintln(f, "## 💰 Pricing Comparison")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Top models by pricing (sorted by input cost):")
	fmt.Fprintln(f)
	writePricingComparisonTable(f, models)
	fmt.Fprintln(f)

	// Add Context Limits Comparison section
	fmt.Fprintln(f, "## 📏 Context Window Comparison")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Top models by context window size:")
	fmt.Fprintln(f)
	writeContextLimitsTable(f, models)
	fmt.Fprintln(f)

	// Add Feature Comparison section
	fmt.Fprintln(f, "## 🎯 Feature Comparison")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "Detailed feature breakdown across models:")
	fmt.Fprintln(f)
	writeFeatureComparisonTable(f, models)
	fmt.Fprintln(f)

	// Footer
	g.writeFooter(f, Breadcrumb{Label: "Back to Catalog", Path: "../"})

	return nil
}