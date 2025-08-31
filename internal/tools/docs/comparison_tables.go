package docs

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	md "github.com/nao1215/markdown"
)

// Comparison table functions for comparing multiple models or providers
// These are useful for index pages and comparison views

// writeModelsOverviewTable generates a comprehensive models overview table
func writeModelsOverviewTable(w io.Writer, models []*catalogs.Model, providers []*catalogs.Provider) {
	builder := NewMarkdownBuilder(w)
	
	// Create provider map for lookup
	providerMap := make(map[string]*catalogs.Provider)
	for _, p := range providers {
		if p.Models != nil {
			// Use sorted models for deterministic iteration
			sortedModels := SortedModels(p.Models)
			for _, model := range sortedModels {
				providerMap[model.ID] = p
			}
		}
	}

	// Make a copy to avoid modifying the original
	modelsCopy := make([]*catalogs.Model, len(models))
	copy(modelsCopy, models)
	
	// Sort models by name
	sort.Slice(modelsCopy, func(i, j int) bool {
		return modelsCopy[i].Name < modelsCopy[j].Name
	})

	// Build table rows
	rows := [][]string{}
	displayCount := min(20, len(modelsCopy))
	
	for _, model := range modelsCopy[:displayCount] {
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

		rows = append(rows, []string{
			"**" + model.Name + "**",
			providerName,
			contextStr,
			inputPrice,
			outputPrice,
			featuresStr,
		})
	}

	if len(modelsCopy) > 20 {
		rows = append(rows, []string{
			fmt.Sprintf("_...and %d more_", len(modelsCopy)-20),
			"", "", "", "", "",
		})
	}

	builder.Table(md.TableSet{
		Header: []string{"Model", "Provider", "Context", "Input", "Output", "Features"},
		Rows:   rows,
	}).Build()
}

// writeProviderComparisonTable generates a provider comparison table
func writeProviderComparisonTable(w io.Writer, providers []*catalogs.Provider) {
	builder := NewMarkdownBuilder(w)
	
	rows := [][]string{}
	
	for _, provider := range providers {
		modelCount := 0
		if provider.Models != nil {
			modelCount = len(provider.Models)
		}

		freeTier := "❌"
		hasFreeModels := false
		// Use sorted models for deterministic iteration
		sortedModels := SortedModels(provider.Models)
		for _, model := range sortedModels {
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
			// Build status page link
			statusBuilder := NewMarkdownBuilderBuffer()
			statusBuilder.Link("Status", *provider.StatusPageURL)
			statusBuilder.Build()
			statusPage = statusBuilder.String()
		}

		rows = append(rows, []string{
			"**" + provider.Name + "**",
			fmt.Sprintf("%d", modelCount),
			freeTier,
			apiKeyRequired,
			statusPage,
		})
	}

	builder.Table(md.TableSet{
		Header: []string{"Provider", "Models", "Free Tier", "API Key Required", "Status Page"},
		Rows:   rows,
	}).Build()
}

// writePricingComparisonTable generates a pricing comparison table for multiple models
func writePricingComparisonTable(w io.Writer, models []*catalogs.Model) {
	builder := NewMarkdownBuilder(w)
	
	// Make a copy to avoid modifying the original
	modelsCopy := make([]*catalogs.Model, len(models))
	copy(modelsCopy, models)
	
	// Sort by input price
	sort.Slice(modelsCopy, func(i, j int) bool {
		iPrice := float64(0)
		jPrice := float64(0)
		if modelsCopy[i].Pricing != nil && modelsCopy[i].Pricing.Tokens != nil && modelsCopy[i].Pricing.Tokens.Input != nil {
			iPrice = modelsCopy[i].Pricing.Tokens.Input.Per1M
		}
		if modelsCopy[j].Pricing != nil && modelsCopy[j].Pricing.Tokens != nil && modelsCopy[j].Pricing.Tokens.Input != nil {
			jPrice = modelsCopy[j].Pricing.Tokens.Input.Per1M
		}
		return iPrice < jPrice
	})

	rows := [][]string{}
	displayCount := min(15, len(modelsCopy))
	
	for _, model := range modelsCopy[:displayCount] {
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

		rows = append(rows, []string{
			"**" + model.Name + "**",
			inputPrice,
			outputPrice,
			cacheRead,
			cacheWrite,
		})
	}

	builder.Table(md.TableSet{
		Header: []string{"Model", "Input (per 1M)", "Output (per 1M)", "Cache Read", "Cache Write"},
		Rows:   rows,
	}).Build()
}

// writeContextLimitsTable generates a context limits comparison table
func writeContextLimitsTable(w io.Writer, models []*catalogs.Model) {
	builder := NewMarkdownBuilder(w)
	
	// Make a copy to avoid modifying the original
	modelsCopy := make([]*catalogs.Model, len(models))
	copy(modelsCopy, models)
	
	// Sort by context window size
	sort.Slice(modelsCopy, func(i, j int) bool {
		iContext := int64(0)
		jContext := int64(0)
		if modelsCopy[i].Limits != nil {
			iContext = modelsCopy[i].Limits.ContextWindow
		}
		if modelsCopy[j].Limits != nil {
			jContext = modelsCopy[j].Limits.ContextWindow
		}
		return iContext > jContext
	})

	rows := [][]string{}
	displayCount := min(15, len(modelsCopy))
	
	for _, model := range modelsCopy[:displayCount] {
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

		rows = append(rows, []string{
			"**" + model.Name + "**",
			contextWindow,
			maxOutput,
			modalityStr,
		})
	}

	builder.Table(md.TableSet{
		Header: []string{"Model", "Context Window", "Max Output", "Modalities"},
		Rows:   rows,
	}).Build()
}

// writeFeatureComparisonTable generates a detailed feature comparison table
func writeFeatureComparisonTable(w io.Writer, models []*catalogs.Model) {
	if len(models) == 0 {
		return
	}

	builder := NewMarkdownBuilder(w)
	builder.H3("Feature Comparison").LF()

	// Make a copy to avoid modifying the original
	modelsCopy := make([]*catalogs.Model, len(models))
	copy(modelsCopy, models)
	
	// Sort models by name for consistency
	sort.Slice(modelsCopy, func(i, j int) bool {
		return modelsCopy[i].Name < modelsCopy[j].Name
	})

	// Limit to 15 models for readability
	displayModels := modelsCopy
	if len(modelsCopy) > 15 {
		displayModels = modelsCopy[:15]
	}

	rows := [][]string{}
	
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
			// Join tools list
			toolsBuilder := NewMarkdownBuilderBuffer()
			toolsBuilder.JoinList(tools, ", ")
			toolsBuilder.Build()
			toolsStr = toolsBuilder.String()
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
			// Join reasoning list
			reasoningBuilder := NewMarkdownBuilderBuffer()
			reasoningBuilder.JoinList(reasoning, ", ")
			reasoningBuilder.Build()
			reasoningStr = reasoningBuilder.String()
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
			// Join advanced list
			advancedBuilder := NewMarkdownBuilderBuffer()
			advancedBuilder.JoinList(advanced, ", ")
			advancedBuilder.Build()
			advancedStr = advancedBuilder.String()
		}
		
		rows = append(rows, []string{
			model.Name,
			modalities,
			toolsStr,
			reasoningStr,
			advancedStr,
		})
	}
	
	if len(models) > 15 {
		rows = append(rows, []string{
			fmt.Sprintf("_...and %d more_", len(models)-15),
			"", "", "", "",
		})
	}
	
	builder.Table(md.TableSet{
		Header: []string{"Model", "Modalities", "Tools", "Reasoning", "Advanced Controls"},
		Rows:   rows,
	}).LF().Build()
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
		// Format as GB
		builder := NewMarkdownBuilderBuffer()
		builder.PlainTextf("%.1f GB", float64(bytes)/float64(GB))
		builder.Build()
		return builder.String()
	case bytes >= MB:
		// Format as MB
		builder := NewMarkdownBuilderBuffer()
		builder.PlainTextf("%.1f MB", float64(bytes)/float64(MB))
		builder.Build()
		return builder.String()
	case bytes >= KB:
		// Format as KB
		builder := NewMarkdownBuilderBuffer()
		builder.PlainTextf("%.1f KB", float64(bytes)/float64(KB))
		builder.Build()
		return builder.String()
	default:
		// Format as bytes
		builder := NewMarkdownBuilderBuffer()
		builder.PlainTextf("%d B", bytes)
		builder.Build()
		return builder.String()
	}
}