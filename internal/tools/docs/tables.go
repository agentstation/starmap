package docs

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// writeModalityTable generates a horizontal table showing modality support for input/output
func writeModalityTable(f *os.File, model *catalogs.Model) {
	if model.Features == nil {
		fmt.Fprintln(f, "No modality information available.")
		fmt.Fprintln(f)
		return
	}

	fmt.Fprintln(f, "| Direction | Text | Image | Audio | Video | PDF |")
	fmt.Fprintln(f, "|-----------|------|-------|-------|-------|-----|")

	// Input row
	fmt.Fprint(f, "| **Input** |")
	allModalities := []catalogs.ModelModality{
		catalogs.ModelModalityText,
		catalogs.ModelModalityImage,
		catalogs.ModelModalityAudio,
		catalogs.ModelModalityVideo,
		catalogs.ModelModalityPDF,
	}

	for _, modality := range allModalities {
		hasModality := false
		for _, inputModality := range model.Features.Modalities.Input {
			if inputModality == modality {
				hasModality = true
				break
			}
		}
		if hasModality {
			fmt.Fprint(f, " ✅ |")
		} else {
			fmt.Fprint(f, " ❌ |")
		}
	}
	fmt.Fprintln(f)

	// Output row
	fmt.Fprint(f, "| **Output** |")
	for _, modality := range allModalities {
		hasModality := false
		for _, outputModality := range model.Features.Modalities.Output {
			if outputModality == modality {
				hasModality = true
				break
			}
		}
		if hasModality {
			fmt.Fprint(f, " ✅ |")
		} else {
			fmt.Fprint(f, " ❌ |")
		}
	}
	fmt.Fprintln(f)
	fmt.Fprintln(f)
}

// writeCoreFeatureTable generates a horizontal table for core features
func writeCoreFeatureTable(f *os.File, model *catalogs.Model) {
	if model.Features == nil {
		fmt.Fprintln(f, "No feature information available.")
		fmt.Fprintln(f)
		return
	}

	fmt.Fprintln(f, "| Tool Calling | Tool Definitions | Tool Choice | Web Search | File Attachments |")
	fmt.Fprintln(f, "|--------------|------------------|-------------|------------|------------------|")
	fmt.Fprint(f, "| ")

	// Tool Calling
	if model.Features.ToolCalls {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// Tool Definitions
	if model.Features.Tools {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// Tool Choice
	if model.Features.ToolChoice {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// Web Search
	if model.Features.WebSearch {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// File Attachments
	if model.Features.Attachments {
		fmt.Fprintln(f, "✅ |")
	} else {
		fmt.Fprintln(f, "❌ |")
	}
	fmt.Fprintln(f)
}

// writeResponseDeliveryTable generates a horizontal table for response delivery options
func writeResponseDeliveryTable(f *os.File, model *catalogs.Model) {
	if model.Features == nil {
		fmt.Fprintln(f, "No delivery information available.")
		fmt.Fprintln(f)
		return
	}

	fmt.Fprintln(f, "| Streaming | Structured Output | JSON Mode | Function Call | Text Format |")
	fmt.Fprintln(f, "|-----------|-------------------|-----------|---------------|-------------|")
	fmt.Fprint(f, "| ")

	// Streaming
	if model.Features.Streaming {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// Structured Output
	if model.Features.StructuredOutputs {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// JSON Mode (check for format response)
	if model.Features.FormatResponse {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// Function Call (same as tool calls)
	if model.Features.ToolCalls {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	// Text Format (always supported if model exists)
	fmt.Fprintln(f, "✅ |")
	fmt.Fprintln(f)
}

// writeAdvancedReasoningTable generates a horizontal table for reasoning capabilities
func writeAdvancedReasoningTable(f *os.File, model *catalogs.Model) {
	if model.Features == nil {
		return
	}

	// Only show this table if any reasoning features are present
	hasReasoningFeatures := model.Features.Reasoning || model.Features.ReasoningEffort ||
		model.Features.ReasoningTokens || model.Features.IncludeReasoning || model.Features.Verbosity

	if !hasReasoningFeatures {
		return
	}

	fmt.Fprintln(f, "### Advanced Reasoning")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Basic Reasoning | Reasoning Effort | Reasoning Tokens | Include Reasoning | Verbosity Control |")
	fmt.Fprintln(f, "|-----------------|------------------|------------------|-------------------|-------------------|")
	fmt.Fprint(f, "| ")

	if model.Features.Reasoning {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	if model.Features.ReasoningEffort {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	if model.Features.ReasoningTokens {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	if model.Features.IncludeReasoning {
		fmt.Fprint(f, "✅ | ")
	} else {
		fmt.Fprint(f, "❌ | ")
	}

	if model.Features.Verbosity {
		fmt.Fprintln(f, "✅ |")
	} else {
		fmt.Fprintln(f, "❌ |")
	}
	fmt.Fprintln(f)
}

// writeControlsTables generates multiple horizontal tables for generation controls
func writeControlsTables(f *os.File, model *catalogs.Model) {
	if model.Features == nil {
		fmt.Fprintln(f, "No control information available.")
		fmt.Fprintln(f)
		return
	}

	// Sampling & Decoding Controls
	hasCoreSampling := model.Features.Temperature || model.Features.TopP || model.Features.TopK ||
		model.Features.TopA || model.Features.MinP

	if hasCoreSampling {
		fmt.Fprintln(f, "### Sampling & Decoding")
		fmt.Fprintln(f)

		// Build table headers dynamically based on what's supported
		var headers []string
		var values []string

		if model.Features.Temperature {
			headers = append(headers, "Temperature")
			rangeStr := ""
			if model.Generation != nil && model.Generation.Temperature != nil {
				rangeStr = fmt.Sprintf("%.1f-%.1f", model.Generation.Temperature.Min, model.Generation.Temperature.Max)
			} else {
				rangeStr = "0.0-2.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.TopP {
			headers = append(headers, "Top-P")
			rangeStr := ""
			if model.Generation != nil && model.Generation.TopP != nil {
				rangeStr = fmt.Sprintf("%.1f-%.1f", model.Generation.TopP.Min, model.Generation.TopP.Max)
			} else {
				rangeStr = "0.0-1.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.TopK {
			headers = append(headers, "Top-K")
			rangeStr := ""
			if model.Generation != nil && model.Generation.TopK != nil {
				rangeStr = fmt.Sprintf("%d-%d", model.Generation.TopK.Min, model.Generation.TopK.Max)
			} else {
				rangeStr = "✅"
			}
			values = append(values, rangeStr)
		}

		if model.Features.TopA {
			headers = append(headers, "Top-A")
			values = append(values, "✅")
		}

		if model.Features.MinP {
			headers = append(headers, "Min-P")
			values = append(values, "✅")
		}

		// Build the table
		fmt.Fprintln(f, "| " + strings.Join(headers, " | ") + " |")
		fmt.Fprintln(f, "|" + strings.Repeat("---|", len(headers)))
		fmt.Fprintln(f, "| " + strings.Join(values, " | ") + " |")
		fmt.Fprintln(f)
	}

	// Length & Termination Controls
	hasLengthControls := model.Features.MaxTokens || model.Features.Stop

	if hasLengthControls {
		fmt.Fprintln(f, "### Length & Termination")
		fmt.Fprintln(f)

		var headers []string
		var values []string

		if model.Features.MaxTokens {
			headers = append(headers, "Max Tokens")
			rangeStr := ""
			if model.Limits != nil && model.Limits.OutputTokens > 0 {
				rangeStr = fmt.Sprintf("1-%s", formatNumber(int(model.Limits.OutputTokens)))
			} else {
				rangeStr = "✅"
			}
			values = append(values, rangeStr)
		}

		if model.Features.Stop {
			headers = append(headers, "Stop Sequences")
			values = append(values, "✅")
		}

		// Build the table
		fmt.Fprintln(f, "| " + strings.Join(headers, " | ") + " |")
		fmt.Fprintln(f, "|" + strings.Repeat("---|", len(headers)))
		fmt.Fprintln(f, "| " + strings.Join(values, " | ") + " |")
		fmt.Fprintln(f)
	}

	// Repetition Control
	hasRepetitionControls := model.Features.FrequencyPenalty || model.Features.PresencePenalty ||
		model.Features.RepetitionPenalty

	if hasRepetitionControls {
		fmt.Fprintln(f, "### Repetition Control")
		fmt.Fprintln(f)

		var headers []string
		var values []string

		if model.Features.FrequencyPenalty {
			headers = append(headers, "Frequency Penalty")
			rangeStr := ""
			if model.Generation != nil && model.Generation.FrequencyPenalty != nil {
				rangeStr = fmt.Sprintf("%.1f to %.1f", model.Generation.FrequencyPenalty.Min, model.Generation.FrequencyPenalty.Max)
			} else {
				rangeStr = "-2.0 to 2.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.PresencePenalty {
			headers = append(headers, "Presence Penalty")
			rangeStr := ""
			if model.Generation != nil && model.Generation.PresencePenalty != nil {
				rangeStr = fmt.Sprintf("%.1f to %.1f", model.Generation.PresencePenalty.Min, model.Generation.PresencePenalty.Max)
			} else {
				rangeStr = "-2.0 to 2.0"
			}
			values = append(values, rangeStr)
		}

		if model.Features.RepetitionPenalty {
			headers = append(headers, "Repetition Penalty")
			values = append(values, "✅")
		}

		// Build the table
		fmt.Fprintln(f, "| " + strings.Join(headers, " | ") + " |")
		fmt.Fprintln(f, "|" + strings.Repeat("---|", len(headers)))
		fmt.Fprintln(f, "| " + strings.Join(values, " | ") + " |")
		fmt.Fprintln(f)
	}

	// Advanced Controls
	hasAdvancedControls := model.Features.LogitBias || model.Features.Seed || model.Features.Logprobs

	if hasAdvancedControls {
		fmt.Fprintln(f, "### Advanced Controls")
		fmt.Fprintln(f)

		var headers []string
		var values []string

		if model.Features.LogitBias {
			headers = append(headers, "Logit Bias")
			values = append(values, "✅")
		}

		if model.Features.Seed {
			headers = append(headers, "Deterministic Seed")
			values = append(values, "✅")
		}

		if model.Features.Logprobs {
			headers = append(headers, "Log Probabilities")
			rangeStr := ""
			if model.Generation != nil && model.Generation.TopLogprobs != nil {
				rangeStr = fmt.Sprintf("0-%d", *model.Generation.TopLogprobs)
			} else {
				rangeStr = "0-20"
			}
			values = append(values, rangeStr)
		}

		// Build the table
		fmt.Fprintln(f, "| " + strings.Join(headers, " | ") + " |")
		fmt.Fprintln(f, "|" + strings.Repeat("---|", len(headers)))
		fmt.Fprintln(f, "| " + strings.Join(values, " | ") + " |")
		fmt.Fprintln(f)
	}
}

// writeArchitectureTable generates a horizontal table for architecture details
func writeArchitectureTable(f *os.File, model *catalogs.Model) {
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return
	}

	arch := model.Metadata.Architecture
	fmt.Fprintln(f, "### Architecture Details")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Parameter Count | Architecture Type | Tokenizer | Quantization | Fine-Tuned | Base Model |")
	fmt.Fprintln(f, "|-----------------|-------------------|-----------|--------------|------------|------------|")
	fmt.Fprint(f, "| ")

	if arch.ParameterCount != "" {
		fmt.Fprintf(f, "%s | ", arch.ParameterCount)
	} else {
		fmt.Fprint(f, "Unknown | ")
	}

	if arch.Type != "" {
		fmt.Fprintf(f, "%s | ", string(arch.Type))
	} else {
		fmt.Fprint(f, "Unknown | ")
	}

	if arch.Tokenizer != "" {
		fmt.Fprintf(f, "%s | ", string(arch.Tokenizer))
	} else {
		fmt.Fprint(f, "Unknown | ")
	}

	if arch.Quantization != "" {
		fmt.Fprintf(f, "%s | ", string(arch.Quantization))
	} else {
		fmt.Fprint(f, "None | ")
	}

	if arch.FineTuned {
		fmt.Fprint(f, "Yes | ")
	} else {
		fmt.Fprint(f, "No | ")
	}

	if arch.BaseModel != nil && *arch.BaseModel != "" {
		fmt.Fprintf(f, "%s |", *arch.BaseModel)
	} else {
		fmt.Fprint(f, "- |")
	}
	fmt.Fprintln(f)
	fmt.Fprintln(f)
}

// writeTagsTable generates a horizontal table for model tags
func writeTagsTable(f *os.File, model *catalogs.Model) {
	if model.Metadata == nil || len(model.Metadata.Tags) == 0 {
		return
	}

	fmt.Fprintln(f, "### Model Tags")
	fmt.Fprintln(f)

	// Common tags to check for
	commonTags := []catalogs.ModelTag{
		catalogs.ModelTagCoding,
		catalogs.ModelTagWriting,
		catalogs.ModelTagReasoning,
		catalogs.ModelTagMath,
		catalogs.ModelTagChat,
		catalogs.ModelTagMultimodal,
		catalogs.ModelTagFunctionCalling,
	}

	// Create header
	fmt.Fprintln(f, "| Coding | Writing | Reasoning | Math | Chat | Multimodal | Function Calling |")
	fmt.Fprintln(f, "|--------|---------|-----------|------|------|------------|------------------|")
	fmt.Fprint(f, "| ")

	// Check each common tag
	for i, tag := range commonTags {
		hasTag := false
		for _, modelTag := range model.Metadata.Tags {
			if modelTag == tag {
				hasTag = true
				break
			}
		}
		if hasTag {
			fmt.Fprint(f, "✅")
		} else {
			fmt.Fprint(f, "❌")
		}
		if i < len(commonTags)-1 {
			fmt.Fprint(f, " | ")
		} else {
			fmt.Fprintln(f, " |")
		}
	}
	fmt.Fprintln(f)

	// Add any additional tags not in the common list
	var additionalTags []string
	for _, modelTag := range model.Metadata.Tags {
		isCommon := false
		for _, commonTag := range commonTags {
			if modelTag == commonTag {
				isCommon = true
				break
			}
		}
		if !isCommon {
			additionalTags = append(additionalTags, string(modelTag))
		}
	}

	if len(additionalTags) > 0 {
		fmt.Fprintf(f, "\n**Additional Tags**: %s\n", strings.Join(additionalTags, ", "))
	}
	fmt.Fprintln(f)
}

// writeTokenPricingTable generates a horizontal table for token pricing
func writeTokenPricingTable(f *os.File, model *catalogs.Model) {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		fmt.Fprintln(f, "Contact provider for pricing information.")
		fmt.Fprintln(f)
		return
	}

	fmt.Fprintln(f, "### Token Pricing")
	fmt.Fprintln(f)

	tokens := model.Pricing.Tokens
	currencySymbol := getCurrencySymbol(model.Pricing.Currency)

	fmt.Fprintln(f, "| Input | Output | Reasoning | Cache Read | Cache Write |")
	fmt.Fprintln(f, "|-------|--------|-----------|------------|-------------|")
	fmt.Fprint(f, "| ")

	if tokens.Input != nil && tokens.Input.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M | ", currencySymbol, tokens.Input.Per1M)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if tokens.Output != nil && tokens.Output.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M | ", currencySymbol, tokens.Output.Per1M)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if tokens.Reasoning != nil && tokens.Reasoning.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M | ", currencySymbol, tokens.Reasoning.Per1M)
	} else {
		fmt.Fprint(f, "- | ")
	}

	// Check both flat structure and nested cache structure
	if tokens.CacheRead != nil && tokens.CacheRead.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M | ", currencySymbol, tokens.CacheRead.Per1M)
	} else if tokens.Cache != nil && tokens.Cache.Read != nil && tokens.Cache.Read.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M | ", currencySymbol, tokens.Cache.Read.Per1M)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if tokens.CacheWrite != nil && tokens.CacheWrite.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M |", currencySymbol, tokens.CacheWrite.Per1M)
	} else if tokens.Cache != nil && tokens.Cache.Write != nil && tokens.Cache.Write.Per1M > 0 {
		fmt.Fprintf(f, "%s%.2f/1M |", currencySymbol, tokens.Cache.Write.Per1M)
	} else {
		fmt.Fprint(f, "- |")
	}
	fmt.Fprintln(f)
	fmt.Fprintln(f)
}

// writeOperationPricingTable generates a horizontal table for operation pricing
func writeOperationPricingTable(f *os.File, model *catalogs.Model) {
	if model.Pricing == nil || model.Pricing.Operations == nil {
		return
	}

	ops := model.Pricing.Operations
	hasOperations := ops.ImageInput != nil || ops.AudioInput != nil || ops.VideoInput != nil ||
		ops.ImageGen != nil || ops.AudioGen != nil || ops.WebSearch != nil

	if !hasOperations {
		return
	}

	fmt.Fprintln(f, "### Operation Pricing")
	fmt.Fprintln(f)

	currencySymbol := getCurrencySymbol(model.Pricing.Currency)

	fmt.Fprintln(f, "| Image Input | Audio Input | Video Input | Image Gen | Audio Gen | Web Search |")
	fmt.Fprintln(f, "|-------------|-------------|-------------|-----------|-----------|------------|")
	fmt.Fprint(f, "| ")

	if ops.ImageInput != nil {
		fmt.Fprintf(f, "%s%.3f/img | ", currencySymbol, *ops.ImageInput)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if ops.AudioInput != nil {
		fmt.Fprintf(f, "%s%.3f/min | ", currencySymbol, *ops.AudioInput)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if ops.VideoInput != nil {
		fmt.Fprintf(f, "%s%.3f/min | ", currencySymbol, *ops.VideoInput)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if ops.ImageGen != nil {
		fmt.Fprintf(f, "%s%.3f/img | ", currencySymbol, *ops.ImageGen)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if ops.AudioGen != nil {
		fmt.Fprintf(f, "%s%.3f/min | ", currencySymbol, *ops.AudioGen)
	} else {
		fmt.Fprint(f, "- | ")
	}

	if ops.WebSearch != nil {
		fmt.Fprintf(f, "%s%.3f/query |", currencySymbol, *ops.WebSearch)
	} else {
		fmt.Fprint(f, "- |")
	}
	fmt.Fprintln(f)
	fmt.Fprintln(f)
}

// getCurrencySymbol returns the currency symbol for a given currency code
func getCurrencySymbol(currency string) string {
	switch currency {
	case "USD", "":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	default:
		return "$" // Default to USD
	}
}