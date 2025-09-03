package docs

import (
	"fmt"
	"io"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	md "github.com/nao1215/markdown"
)

// writeModalityTable generates a horizontal table showing modality support for input/output
func writeModalityTable(w io.Writer, model *catalogs.Model) {
	markdown := NewMarkdown(w)

	if model.Features == nil {
		markdown.PlainText("No modality information available.").LF().Build()
		return
	}

	allModalities := []catalogs.ModelModality{
		catalogs.ModelModalityText,
		catalogs.ModelModalityImage,
		catalogs.ModelModalityAudio,
		catalogs.ModelModalityVideo,
		catalogs.ModelModalityPDF,
	}

	// Build input row
	inputRow := []string{"**Input**"}
	for _, modality := range allModalities {
		hasModality := false
		for _, inputModality := range model.Features.Modalities.Input {
			if inputModality == modality {
				hasModality = true
				break
			}
		}
		if hasModality {
			inputRow = append(inputRow, "✅")
		} else {
			inputRow = append(inputRow, "❌")
		}
	}

	// Build output row
	outputRow := []string{"**Output**"}
	for _, modality := range allModalities {
		hasModality := false
		for _, outputModality := range model.Features.Modalities.Output {
			if outputModality == modality {
				hasModality = true
				break
			}
		}
		if hasModality {
			outputRow = append(outputRow, "✅")
		} else {
			outputRow = append(outputRow, "❌")
		}
	}

	markdown.Table(md.TableSet{
		Header: []string{"Direction", "Text", "Image", "Audio", "Video", "PDF"},
		Rows:   [][]string{inputRow, outputRow},
	}).LF().Build()
}

// writeCoreFeatureTable generates a horizontal table for core features
func writeCoreFeatureTable(w io.Writer, model *catalogs.Model) {
	markdown := NewMarkdown(w)

	if model.Features == nil {
		markdown.PlainText("No feature information available.").LF().Build()
		return
	}

	row := []string{}

	// Tool Calling
	if model.Features.ToolCalls {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// Tool Definitions
	if model.Features.Tools {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// Tool Choice
	if model.Features.ToolChoice {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// Web Search
	if model.Features.WebSearch {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// File Attachments
	if model.Features.Attachments {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	markdown.Table(md.TableSet{
		Header: []string{"Tool Calling", "Tool Definitions", "Tool Choice", "Web Search", "File Attachments"},
		Rows:   [][]string{row},
	}).LF().Build()
}

// writeResponseDeliveryTable generates a horizontal table for response delivery options
func writeResponseDeliveryTable(w io.Writer, model *catalogs.Model) {
	markdown := NewMarkdown(w)

	if model.Features == nil {
		markdown.PlainText("No delivery information available.").LF().Build()
		return
	}

	row := []string{}

	// Streaming
	if model.Features.Streaming {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// Structured Output
	if model.Features.StructuredOutputs {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// JSON Mode (check for format response)
	if model.Features.FormatResponse {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// Function Call (same as tool calls)
	if model.Features.ToolCalls {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	// Text Format (always supported if model exists)
	row = append(row, "✅")

	markdown.Table(md.TableSet{
		Header: []string{"Streaming", "Structured Output", "JSON Mode", "Function Call", "Text Format"},
		Rows:   [][]string{row},
	}).LF().Build()
}

// writeAdvancedReasoningTable generates a horizontal table for reasoning capabilities
func writeAdvancedReasoningTable(w io.Writer, model *catalogs.Model) {
	if model.Features == nil {
		return
	}

	// Only show this table if any reasoning features are present
	hasReasoningFeatures := model.Features.Reasoning || model.Features.ReasoningEffort ||
		model.Features.ReasoningTokens || model.Features.IncludeReasoning || model.Features.Verbosity

	if !hasReasoningFeatures {
		return
	}

	markdown := NewMarkdown(w)
	markdown.H3("Advanced Reasoning").LF()

	row := []string{}

	if model.Features.Reasoning {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	if model.Features.ReasoningEffort {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	if model.Features.ReasoningTokens {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	if model.Features.IncludeReasoning {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	if model.Features.Verbosity {
		row = append(row, "✅")
	} else {
		row = append(row, "❌")
	}

	markdown.Table(md.TableSet{
		Header: []string{"Basic Reasoning", "Reasoning Effort", "Reasoning Tokens", "Include Reasoning", "Verbosity Control"},
		Rows:   [][]string{row},
	}).LF().Build()
}

// writeControlsTables generates multiple horizontal tables for generation controls
func writeControlsTables(w io.Writer, model *catalogs.Model) {
	if model.Features == nil {
		markdown := NewMarkdown(w)
		markdown.PlainText("No control information available.").LF().Build()
		return
	}

	markdown := NewMarkdown(w)

	// Sampling & Decoding Controls
	hasCoreSampling := model.Features.Temperature || model.Features.TopP || model.Features.TopK ||
		model.Features.TopA || model.Features.MinP

	if hasCoreSampling {
		markdown.H3("Sampling & Decoding").LF()

		// Build table headers and values dynamically based on what's supported
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

		markdown.Table(md.TableSet{
			Header: headers,
			Rows:   [][]string{values},
		}).LF()
	}

	// Length & Termination Controls
	hasLengthControls := model.Features.MaxTokens || model.Features.Stop

	if hasLengthControls {
		markdown.H3("Length & Termination").LF()

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

		markdown.Table(md.TableSet{
			Header: headers,
			Rows:   [][]string{values},
		}).LF()
	}

	// Repetition Control
	hasRepetitionControls := model.Features.FrequencyPenalty || model.Features.PresencePenalty ||
		model.Features.RepetitionPenalty

	if hasRepetitionControls {
		markdown.H3("Repetition Control").LF()

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

		markdown.Table(md.TableSet{
			Header: headers,
			Rows:   [][]string{values},
		}).LF()
	}

	// Advanced Controls
	hasAdvancedControls := model.Features.LogitBias || model.Features.Seed || model.Features.Logprobs

	if hasAdvancedControls {
		markdown.H3("Advanced Controls").LF()

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

		markdown.Table(md.TableSet{
			Header: headers,
			Rows:   [][]string{values},
		}).LF()
	}

	markdown.Build()
}

// writeArchitectureTable generates a horizontal table for architecture details
func writeArchitectureTable(w io.Writer, model *catalogs.Model) {
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return
	}

	markdown := NewMarkdown(w)
	arch := model.Metadata.Architecture

	markdown.H3("Architecture Details").LF()

	row := []string{}

	if arch.ParameterCount != "" {
		row = append(row, arch.ParameterCount)
	} else {
		row = append(row, "Unknown")
	}

	if arch.Type != "" {
		row = append(row, string(arch.Type))
	} else {
		row = append(row, "Unknown")
	}

	if arch.Tokenizer != "" {
		row = append(row, string(arch.Tokenizer))
	} else {
		row = append(row, "Unknown")
	}

	if arch.Quantization != "" {
		row = append(row, string(arch.Quantization))
	} else {
		row = append(row, "None")
	}

	if arch.FineTuned {
		row = append(row, "Yes")
	} else {
		row = append(row, "No")
	}

	if arch.BaseModel != nil && *arch.BaseModel != "" {
		row = append(row, *arch.BaseModel)
	} else {
		row = append(row, "-")
	}

	markdown.Table(md.TableSet{
		Header: []string{"Parameter Count", "Architecture Type", "Tokenizer", "Quantization", "Fine-Tuned", "Base Model"},
		Rows:   [][]string{row},
	}).LF().Build()
}

// writeTagsTable generates a horizontal table for model tags
func writeTagsTable(w io.Writer, model *catalogs.Model) {
	if model.Metadata == nil || len(model.Metadata.Tags) == 0 {
		return
	}

	markdown := NewMarkdown(w)
	markdown.H3("Model Tags").LF()

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

	row := []string{}

	// Check each common tag
	for _, tag := range commonTags {
		hasTag := false
		for _, modelTag := range model.Metadata.Tags {
			if modelTag == tag {
				hasTag = true
				break
			}
		}
		if hasTag {
			row = append(row, "✅")
		} else {
			row = append(row, "❌")
		}
	}

	markdown.Table(md.TableSet{
		Header: []string{"Coding", "Writing", "Reasoning", "Math", "Chat", "Multimodal", "Function Calling"},
		Rows:   [][]string{row},
	}).LF()

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
		markdown.LF().Bold("Additional Tags").PlainText(": " + strings.Join(additionalTags, ", ")).LF()
	}

	markdown.Build()
}

// writeTokenPricingTable generates a horizontal table for token pricing
func writeTokenPricingTable(w io.Writer, model *catalogs.Model) {
	markdown := NewMarkdown(w)

	if model.Pricing == nil || model.Pricing.Tokens == nil {
		markdown.PlainText("Contact provider for pricing information.").LF().Build()
		return
	}

	markdown.H3("Token Pricing").LF()

	tokens := model.Pricing.Tokens
	currencySymbol := model.Pricing.Currency.Symbol()

	row := []string{}

	if tokens.Input != nil && tokens.Input.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.Input.Per1M))
	} else {
		row = append(row, "-")
	}

	if tokens.Output != nil && tokens.Output.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.Output.Per1M))
	} else {
		row = append(row, "-")
	}

	if tokens.Reasoning != nil && tokens.Reasoning.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.Reasoning.Per1M))
	} else {
		row = append(row, "-")
	}

	// Check both flat structure and nested cache structure
	if tokens.CacheRead != nil && tokens.CacheRead.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.CacheRead.Per1M))
	} else if tokens.Cache != nil && tokens.Cache.Read != nil && tokens.Cache.Read.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.Cache.Read.Per1M))
	} else {
		row = append(row, "-")
	}

	if tokens.CacheWrite != nil && tokens.CacheWrite.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.CacheWrite.Per1M))
	} else if tokens.Cache != nil && tokens.Cache.Write != nil && tokens.Cache.Write.Per1M > 0 {
		row = append(row, fmt.Sprintf("%s%.2f/1M", currencySymbol, tokens.Cache.Write.Per1M))
	} else {
		row = append(row, "-")
	}

	markdown.Table(md.TableSet{
		Header: []string{"Input", "Output", "Reasoning", "Cache Read", "Cache Write"},
		Rows:   [][]string{row},
	}).LF().Build()
}

// writeOperationPricingTable generates a horizontal table for operation pricing
func writeOperationPricingTable(w io.Writer, model *catalogs.Model) {
	if model.Pricing == nil || model.Pricing.Operations == nil {
		return
	}

	ops := model.Pricing.Operations
	hasOperations := ops.ImageInput != nil || ops.AudioInput != nil || ops.VideoInput != nil ||
		ops.ImageGen != nil || ops.AudioGen != nil || ops.WebSearch != nil

	if !hasOperations {
		return
	}

	markdown := NewMarkdown(w)
	markdown.H3("Operation Pricing").LF()

	currencySymbol := model.Pricing.Currency.Symbol()

	row := []string{}

	if ops.ImageInput != nil {
		row = append(row, fmt.Sprintf("%s%.3f/img", currencySymbol, *ops.ImageInput))
	} else {
		row = append(row, "-")
	}

	if ops.AudioInput != nil {
		row = append(row, fmt.Sprintf("%s%.3f/min", currencySymbol, *ops.AudioInput))
	} else {
		row = append(row, "-")
	}

	if ops.VideoInput != nil {
		row = append(row, fmt.Sprintf("%s%.3f/min", currencySymbol, *ops.VideoInput))
	} else {
		row = append(row, "-")
	}

	if ops.ImageGen != nil {
		row = append(row, fmt.Sprintf("%s%.3f/img", currencySymbol, *ops.ImageGen))
	} else {
		row = append(row, "-")
	}

	if ops.AudioGen != nil {
		row = append(row, fmt.Sprintf("%s%.3f/min", currencySymbol, *ops.AudioGen))
	} else {
		row = append(row, "-")
	}

	if ops.WebSearch != nil {
		row = append(row, fmt.Sprintf("%s%.3f/query", currencySymbol, *ops.WebSearch))
	} else {
		row = append(row, "-")
	}

	markdown.Table(md.TableSet{
		Header: []string{"Image Input", "Audio Input", "Video Input", "Image Gen", "Audio Gen", "Web Search"},
		Rows:   [][]string{row},
	}).LF().Build()
}
