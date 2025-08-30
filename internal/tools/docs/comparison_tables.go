package docs

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Comparison table functions for comparing multiple models or providers
// These are useful for index pages and comparison views

// writeModelsOverviewTable generates a comprehensive models overview table
func writeModelsOverviewTable(f *os.File, models []*catalogs.Model, providers []*catalogs.Provider) {
	fmt.Fprintln(f, "| Model | Provider | Context | Input | Output | Features |")
	fmt.Fprintln(f, "|-------|----------|---------|-------|--------|----------|")

	// Create provider map for lookup
	providerMap := make(map[string]*catalogs.Provider)
	for _, p := range providers {
		if p.Models != nil {
			for modelID := range p.Models {
				providerMap[modelID] = p
			}
		}
	}

	// Sort models by name
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	for _, model := range models[:min(20, len(models))] {
		// Provider
		providerName := "—"
		if provider, ok := providerMap[model.ID]; ok {
			providerName = provider.Name
		}

		// Context
		contextStr := "—"
		if model.Limits != nil && model.Limits.ContextWindow > 0 {
			contextStr = formatContext(model.Limits.ContextWindow)
		}

		// Pricing
		inputPrice := "—"
		outputPrice := "—"
		if model.Pricing != nil && model.Pricing.Tokens != nil {
			if model.Pricing.Tokens.Input != nil {
				inputPrice = formatPrice(model.Pricing.Tokens.Input.Per1M)
			}
			if model.Pricing.Tokens.Output != nil {
				outputPrice = formatPrice(model.Pricing.Tokens.Output.Per1M)
			}
		}

		// Features - use compactFeatures for consistency
		featuresStr := compactFeatures(*model)

		fmt.Fprintf(f, "| **%s** | %s | %s | %s | %s | %s |\n",
			model.Name, providerName, contextStr, inputPrice, outputPrice, featuresStr)
	}

	if len(models) > 20 {
		fmt.Fprintf(f, "| _...and %d more_ | | | | | |\n", len(models)-20)
	}
}

// writeProviderComparisonTable generates a provider comparison table
func writeProviderComparisonTable(f *os.File, providers []*catalogs.Provider) {
	fmt.Fprintln(f, "| Provider | Models | Free Tier | API Key Required | Status Page |")
	fmt.Fprintln(f, "|----------|--------|-----------|------------------|-------------|")

	for _, provider := range providers {
		modelCount := 0
		if provider.Models != nil {
			modelCount = len(provider.Models)
		}

		freeTier := "❌"
		hasFreeModels := false
		for _, model := range provider.Models {
			if model.Pricing != nil && model.Pricing.Tokens != nil {
				if (model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M == 0) ||
					(model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M == 0) {
					hasFreeModels = true
					break
				}
			}
		}
		if hasFreeModels {
			freeTier = "✅"
		}

		apiKeyRequired := "✅"
		if provider.APIKey != nil {
			// Has API key configuration
			apiKeyRequired = "✅"
		} else if len(provider.EnvVars) > 0 {
			// Uses environment variables
			apiKeyRequired = "✅"
		} else {
			apiKeyRequired = "❌"
		}

		statusPage := "—"
		if provider.StatusPageURL != nil && *provider.StatusPageURL != "" {
			statusPage = fmt.Sprintf("[Status](%s)", *provider.StatusPageURL)
		}

		fmt.Fprintf(f, "| **%s** | %d | %s | %s | %s |\n",
			provider.Name, modelCount, freeTier, apiKeyRequired, statusPage)
	}
}

// writePricingComparisonTable generates a pricing comparison table for multiple models
func writePricingComparisonTable(f *os.File, models []*catalogs.Model) {
	fmt.Fprintln(f, "| Model | Input (per 1M) | Output (per 1M) | Cache Read | Cache Write |")
	fmt.Fprintln(f, "|-------|----------------|-----------------|------------|-------------|")

	// Sort by input price
	sort.Slice(models, func(i, j int) bool {
		iPrice := float64(0)
		jPrice := float64(0)
		if models[i].Pricing != nil && models[i].Pricing.Tokens != nil && models[i].Pricing.Tokens.Input != nil {
			iPrice = models[i].Pricing.Tokens.Input.Per1M
		}
		if models[j].Pricing != nil && models[j].Pricing.Tokens != nil && models[j].Pricing.Tokens.Input != nil {
			jPrice = models[j].Pricing.Tokens.Input.Per1M
		}
		return iPrice < jPrice
	})

	for _, model := range models[:min(15, len(models))] {
		if model.Pricing == nil || model.Pricing.Tokens == nil {
			continue
		}

		inputPrice := "—"
		outputPrice := "—"
		cacheRead := "—"
		cacheWrite := "—"

		tokens := model.Pricing.Tokens
		if tokens.Input != nil {
			inputPrice = formatPrice(tokens.Input.Per1M)
		}
		if tokens.Output != nil {
			outputPrice = formatPrice(tokens.Output.Per1M)
		}
		if tokens.CacheRead != nil {
			cacheRead = formatPrice(tokens.CacheRead.Per1M)
		}
		if tokens.CacheWrite != nil {
			cacheWrite = formatPrice(tokens.CacheWrite.Per1M)
		}

		fmt.Fprintf(f, "| **%s** | %s | %s | %s | %s |\n",
			model.Name, inputPrice, outputPrice, cacheRead, cacheWrite)
	}
}

// writeContextLimitsTable generates a context limits comparison table
func writeContextLimitsTable(f *os.File, models []*catalogs.Model) {
	fmt.Fprintln(f, "| Model | Context Window | Max Output | Modalities |")
	fmt.Fprintln(f, "|-------|---------------|------------|------------|")

	// Sort by context window size
	sort.Slice(models, func(i, j int) bool {
		iContext := int64(0)
		jContext := int64(0)
		if models[i].Limits != nil {
			iContext = models[i].Limits.ContextWindow
		}
		if models[j].Limits != nil {
			jContext = models[j].Limits.ContextWindow
		}
		return iContext > jContext
	})

	for _, model := range models[:min(15, len(models))] {
		if model.Limits == nil {
			continue
		}

		contextWindow := formatContext(model.Limits.ContextWindow)
		maxOutput := "—"
		if model.Limits.OutputTokens > 0 {
			maxOutput = formatNumber(int(model.Limits.OutputTokens))
		}

		// Get modalities instead of max images/file size
		modalities := []string{}
		if model.Features != nil {
			if hasText(model.Features) {
				modalities = append(modalities, "Text")
			}
			if hasVision(model.Features) {
				modalities = append(modalities, "Image")
			}
			if hasAudio(model.Features) {
				modalities = append(modalities, "Audio")
			}
			if hasVideo(model.Features) {
				modalities = append(modalities, "Video")
			}
		}
		modalityStr := "—"
		if len(modalities) > 0 {
			modalityStr = strings.Join(modalities, ", ")
		}

		fmt.Fprintf(f, "| **%s** | %s | %s | %s |\n",
			model.Name, contextWindow, maxOutput, modalityStr)
	}
}

// writeFeatureComparisonTable generates a detailed feature comparison table
func writeFeatureComparisonTable(f *os.File, models []*catalogs.Model) {
	if len(models) == 0 {
		return
	}

	fmt.Fprintln(f, "### Feature Comparison")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Model | Modalities | Tools | Reasoning | Advanced Controls |")
	fmt.Fprintln(f, "|-------|------------|-------|-----------|-------------------|")

	// Sort models by name for consistency
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	// Limit to 15 models for readability
	displayModels := models
	if len(models) > 15 {
		displayModels = models[:15]
	}

	for _, model := range displayModels {
		// Modalities (compact)
		modalities := compactFeatures(*model)
		
		// Tools section
		tools := []string{}
		if model.Features != nil {
			if model.Features.Tools || model.Features.ToolCalls {
				tools = append(tools, "🔧 Tools")
			}
			if model.Features.WebSearch {
				tools = append(tools, "🌐 Search")
			}
			if model.Features.Attachments {
				tools = append(tools, "📎 Files")
			}
		}
		toolsStr := "—"
		if len(tools) > 0 {
			toolsStr = strings.Join(tools, ", ")
		}
		
		// Reasoning section
		reasoning := []string{}
		if model.Features != nil {
			if model.Features.Reasoning {
				reasoning = append(reasoning, "🧠 Basic")
			}
			if model.Features.ReasoningEffort {
				reasoning = append(reasoning, "⚙️ Configurable")
			}
			if model.Features.ReasoningTokens {
				reasoning = append(reasoning, "🎯 Tokens")
			}
		}
		reasoningStr := "—"
		if len(reasoning) > 0 {
			reasoningStr = strings.Join(reasoning, ", ")
		}
		
		// Advanced controls
		advanced := []string{}
		if model.Features != nil {
			if model.Features.Temperature && model.Features.TopP {
				advanced = append(advanced, "🌡️ Sampling")
			}
			if model.Features.Seed {
				advanced = append(advanced, "🎲 Seed")
			}
			if model.Features.Logprobs {
				advanced = append(advanced, "📊 Logprobs")
			}
			if model.Features.FrequencyPenalty || model.Features.PresencePenalty {
				advanced = append(advanced, "🔁 Penalties")
			}
		}
		advancedStr := "—"
		if len(advanced) > 0 {
			advancedStr = strings.Join(advanced, ", ")
		}
		
		fmt.Fprintf(f, "| %s | %s | %s | %s | %s |\n",
			model.Name, modalities, toolsStr, reasoningStr, advancedStr)
	}
	
	if len(models) > 15 {
		fmt.Fprintf(f, "| _...and %d more_ | | | | |\n", len(models)-15)
	}
	
	fmt.Fprintln(f)
}

// Helper function to format bytes into human-readable format
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}