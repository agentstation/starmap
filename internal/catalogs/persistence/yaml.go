package persistence

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// GenerateStructuredModelYAML creates a well-formatted YAML with comments and sections
func GenerateStructuredModelYAML(model catalogs.Model) string {
	var sb strings.Builder

	// Header comment
	sb.WriteString(fmt.Sprintf("# %s - %s\n", model.ID, getModelDescription(model)))

	// Basic identification
	sb.WriteString(fmt.Sprintf("id: %s\n", model.ID))
	if model.Name != "" {
		sb.WriteString(fmt.Sprintf("name: %s\n", model.Name))
	}

	// Authors section
	if len(model.Authors) > 0 {
		sb.WriteString("authors:\n")
		for _, author := range model.Authors {
			if author.Name != "" {
				sb.WriteString(fmt.Sprintf("- id: %s\n  name: %s\n", author.ID, author.Name))
			} else {
				sb.WriteString(fmt.Sprintf("- id: %s\n", author.ID))
			}
		}
	}

	// Description (if available)
	if model.Description != "" {
		sb.WriteString(fmt.Sprintf("description: >\n  %s", model.Description))
	}
	sb.WriteString("\n") // Add a newline after the description or authors

	// Model metadata (early in the file for better visibility)
	if model.Metadata != nil {
		hasMetadata := false
		var metadataSection strings.Builder

		metadataSection.WriteString("# Model metadata\n")
		metadataSection.WriteString("metadata:\n")

		if !model.Metadata.ReleaseDate.IsZero() {
			metadataSection.WriteString(fmt.Sprintf("  release_date: %s\n", model.Metadata.ReleaseDate.Format("2006-01-02")))
			hasMetadata = true
		}
		if model.Metadata.KnowledgeCutoff != nil && !model.Metadata.KnowledgeCutoff.IsZero() {
			metadataSection.WriteString(fmt.Sprintf("  knowledge_cutoff: %s\n", model.Metadata.KnowledgeCutoff.Format("2006-01-02")))
			hasMetadata = true
		}
		// Only write open_weights if it's explicitly set to true (to avoid cluttering with false values)
		if model.Metadata.OpenWeights {
			metadataSection.WriteString(fmt.Sprintf("  open_weights: %t\n", model.Metadata.OpenWeights))
			hasMetadata = true
		}

		if hasMetadata {
			sb.WriteString(metadataSection.String())
			sb.WriteString("\n")
		}
	}

	// Model features section
	if model.Features != nil {
		sb.WriteString("# Model features\n")
		sb.WriteString("features:\n")

		// Modalities
		if len(model.Features.Modalities.Input) > 0 || len(model.Features.Modalities.Output) > 0 {
			sb.WriteString("  modalities:\n")
			if len(model.Features.Modalities.Input) > 0 {
				sb.WriteString("    input:\n")
				for _, modality := range model.Features.Modalities.Input {
					sb.WriteString(fmt.Sprintf("    - %s\n", modality))
				}
			}
			if len(model.Features.Modalities.Output) > 0 {
				sb.WriteString("    output:\n")
				for _, modality := range model.Features.Modalities.Output {
					sb.WriteString(fmt.Sprintf("    - %s\n", modality))
				}
			}
			sb.WriteString("\n")
		}

		// Core capabilities
		sb.WriteString("  # Core capabilities\n")
		sb.WriteString(fmt.Sprintf("  tool_calls: %t\n", model.Features.ToolCalls))
		sb.WriteString(fmt.Sprintf("  tools: %t\n", model.Features.Tools))
		sb.WriteString(fmt.Sprintf("  tool_choice: %t\n", model.Features.ToolChoice))
		sb.WriteString(fmt.Sprintf("  web_search: %t\n", model.Features.WebSearch))
		sb.WriteString(fmt.Sprintf("  attachments: %t\n", model.Features.Attachments))
		sb.WriteString("\n")

		// Reasoning & Verbosity
		sb.WriteString("  # Reasoning & Verbosity\n")
		sb.WriteString(fmt.Sprintf("  reasoning: %t\n", model.Features.Reasoning))
		sb.WriteString(fmt.Sprintf("  reasoning_effort: %t\n", model.Features.ReasoningEffort))
		sb.WriteString(fmt.Sprintf("  reasoning_tokens: %t\n", model.Features.ReasoningTokens))
		sb.WriteString(fmt.Sprintf("  include_reasoning: %t\n", model.Features.IncludeReasoning))
		sb.WriteString(fmt.Sprintf("  verbosity: %t\n", model.Features.Verbosity))
		sb.WriteString("\n")

		// Generation control support flags
		sb.WriteString("  # Generation control support flags\n")
		sb.WriteString(fmt.Sprintf("  temperature: %t\n", model.Features.Temperature))
		sb.WriteString(fmt.Sprintf("  top_p: %t\n", model.Features.TopP))
		sb.WriteString(fmt.Sprintf("  top_k: %t\n", model.Features.TopK))
		sb.WriteString(fmt.Sprintf("  top_a: %t\n", model.Features.TopA))
		sb.WriteString(fmt.Sprintf("  min_p: %t\n", model.Features.MinP))
		sb.WriteString(fmt.Sprintf("  max_tokens: %t\n", model.Features.MaxTokens))
		sb.WriteString(fmt.Sprintf("  frequency_penalty: %t\n", model.Features.FrequencyPenalty))
		sb.WriteString(fmt.Sprintf("  presence_penalty: %t\n", model.Features.PresencePenalty))
		sb.WriteString(fmt.Sprintf("  repetition_penalty: %t\n", model.Features.RepetitionPenalty))
		sb.WriteString(fmt.Sprintf("  logit_bias: %t\n", model.Features.LogitBias))
		sb.WriteString(fmt.Sprintf("  seed: %t\n", model.Features.Seed))
		sb.WriteString(fmt.Sprintf("  stop: %t\n", model.Features.Stop))
		sb.WriteString(fmt.Sprintf("  logprobs: %t\n", model.Features.Logprobs))
		sb.WriteString("\n")

		// Response delivery
		sb.WriteString("  # Response delivery\n")
		sb.WriteString(fmt.Sprintf("  format_response: %t\n", model.Features.FormatResponse))
		sb.WriteString(fmt.Sprintf("  structured_outputs: %t\n", model.Features.StructuredOutputs))
		sb.WriteString(fmt.Sprintf("  streaming: %t\n", model.Features.Streaming))
		sb.WriteString("\n")
	}

	// Model limits
	if model.Limits != nil {
		sb.WriteString("# Model limits\n")
		sb.WriteString("limits:\n")
		if model.Limits.ContextWindow > 0 {
			sb.WriteString(fmt.Sprintf("  context_window: %d\n", model.Limits.ContextWindow))
		}
		if model.Limits.OutputTokens > 0 {
			sb.WriteString(fmt.Sprintf("  output_tokens: %d\n", model.Limits.OutputTokens))
		}
		sb.WriteString("\n")
	}

	// Model pricing
	if model.Pricing != nil {
		sb.WriteString("# Model pricing\n")
		sb.WriteString("pricing:\n")
		if model.Pricing.Currency != "" {
			sb.WriteString(fmt.Sprintf("  currency: %s\n", model.Pricing.Currency))
		}

		if model.Pricing.Tokens != nil {
			sb.WriteString("  tokens:\n")
			if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M > 0 {
				sb.WriteString("    input:\n")
				sb.WriteString(fmt.Sprintf("      per_1m: %.2f\n", model.Pricing.Tokens.Input.Per1M))
			}
			if model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M > 0 {
				sb.WriteString("    output:\n")
				sb.WriteString(fmt.Sprintf("      per_1m: %.2f\n", model.Pricing.Tokens.Output.Per1M))
			}
			if model.Pricing.Tokens.Cache != nil {
				if model.Pricing.Tokens.Cache.Read != nil && model.Pricing.Tokens.Cache.Read.Per1M > 0 {
					sb.WriteString("    cache_read:\n")
					sb.WriteString(fmt.Sprintf("      per_1m: %.2f\n", model.Pricing.Tokens.Cache.Read.Per1M))
				}
				if model.Pricing.Tokens.Cache.Write != nil && model.Pricing.Tokens.Cache.Write.Per1M > 0 {
					sb.WriteString("    cache_write:\n")
					sb.WriteString(fmt.Sprintf("      per_1m: %.2f\n", model.Pricing.Tokens.Cache.Write.Per1M))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Timestamps
	if !model.CreatedAt.IsZero() || !model.UpdatedAt.IsZero() {
		sb.WriteString("# Timestamps\n")
		if !model.CreatedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("created_at: %s\n", model.CreatedAt.Format("2006-01-02T15:04:05Z07:00")))
		}
		if !model.UpdatedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("updated_at: %s\n", model.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")))
		}
	}

	return sb.String()
}

// getModelDescription generates a brief description for the model header comment
func getModelDescription(model catalogs.Model) string {
	if model.Description != "" {
		// Truncate description for header comment
		if len(model.Description) > 60 {
			return model.Description[:57] + "..."
		}
		return model.Description
	}

	if model.Name != "" && model.Name != model.ID {
		return model.Name
	}

	// Generate description based on model ID patterns
	modelID := strings.ToLower(model.ID)
	switch {
	case strings.Contains(modelID, "claude"):
		return "Claude language model"
	case strings.Contains(modelID, "gpt"):
		return "GPT language model"
	case strings.Contains(modelID, "gemini"):
		return "Gemini language model"
	case strings.Contains(modelID, "llama"):
		return "Llama language model"
	case strings.Contains(modelID, "embedding"):
		return "Text embedding model"
	case strings.Contains(modelID, "whisper"):
		return "Speech recognition model"
	case strings.Contains(modelID, "dall-e") || strings.Contains(modelID, "imagen"):
		return "Image generation model"
	case strings.Contains(modelID, "tts"):
		return "Text-to-speech model"
	default:
		return "Language model"
	}
}
