package models

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// printModelDetails prints detailed model information using table format.
func printModelDetails(model *catalogs.Model, provider catalogs.Provider) {
	formatter := format.NewFormatter(format.FormatTable)

	fmt.Printf("Model: %s\n\n", model.ID)

	printBasicInfo(model, provider, formatter)
	printLimitsInfo(model, formatter)
	printPricingInfo(model, formatter)
	printFeaturesInfo(model, formatter)
	printArchitectureInfo(model, formatter)
}

func printBasicInfo(model *catalogs.Model, provider catalogs.Provider, formatter format.Formatter) {
	basicRows := [][]string{
		{"Model ID", model.ID},
		{"Name", model.Name},
		{"Provider", fmt.Sprintf("%s (%s)", provider.Name, provider.ID)},
	}

	if len(model.Authors) > 0 {
		authorNames := make([]string, len(model.Authors))
		for i, author := range model.Authors {
			authorNames[i] = author.Name
		}
		basicRows = append(basicRows, []string{"Authors", strings.Join(authorNames, ", ")})
	} else {
		basicRows = append(basicRows, []string{"Authors", "Unknown"})
	}

	if model.Description != "" {
		description := model.Description
		if len(description) > 80 {
			description = description[:77] + "..."
		}
		basicRows = append(basicRows, []string{"Description", description})
	}

	basicTable := format.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()
}

func printLimitsInfo(model *catalogs.Model, formatter format.Formatter) {
	if model.Limits == nil {
		return
	}

	var limitRows [][]string
	if model.Limits.ContextWindow > 0 {
		limitRows = append(limitRows, []string{"Context Window", fmt.Sprintf("%s tokens", table.FormatNumber(model.Limits.ContextWindow))})
	}
	if model.Limits.OutputTokens > 0 {
		limitRows = append(limitRows, []string{"Max Output", fmt.Sprintf("%s tokens", table.FormatNumber(model.Limits.OutputTokens))})
	}

	if len(limitRows) > 0 {
		limitsTable := format.Data{
			Headers: []string{"Limit", "Value"},
			Rows:    limitRows,
		}
		fmt.Println("Limits:")
		_ = formatter.Format(os.Stdout, limitsTable)
		fmt.Println()
	}
}

func printPricingInfo(model *catalogs.Model, formatter format.Formatter) {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		return
	}

	var pricingRows [][]string
	if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M > 0 {
		pricingRows = append(pricingRows, []string{"Input", fmt.Sprintf("$%.6f per 1M tokens", model.Pricing.Tokens.Input.Per1M)})
	}
	if model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M > 0 {
		pricingRows = append(pricingRows, []string{"Output", fmt.Sprintf("$%.6f per 1M tokens", model.Pricing.Tokens.Output.Per1M)})
	}

	if len(pricingRows) > 0 {
		pricingTable := format.Data{
			Headers: []string{"Type", "Price"},
			Rows:    pricingRows,
		}
		fmt.Println("Pricing:")
		_ = formatter.Format(os.Stdout, pricingTable)
		fmt.Println()
	}
}

func printFeaturesInfo(model *catalogs.Model, formatter format.Formatter) {
	if model.Features == nil {
		return
	}

	var featureRows [][]string

	// Check modality features
	featureRows = addModalityFeatures(featureRows, model.Features)

	// Other features
	if model.Features.ToolCalls {
		featureRows = append(featureRows, []string{"Function Calling", emoji.Success + " Supported"})
	}
	if model.Features.WebSearch {
		featureRows = append(featureRows, []string{"Web Search", emoji.Success + " Supported"})
	}
	if model.Features.Reasoning {
		featureRows = append(featureRows, []string{"Reasoning", emoji.Success + " Supported"})
	}

	if len(featureRows) > 0 {
		featuresTable := format.Data{
			Headers: []string{"Feature", "Status"},
			Rows:    featureRows,
		}
		fmt.Println("Features:")
		_ = formatter.Format(os.Stdout, featuresTable)
		fmt.Println()
	}
}

func addModalityFeatures(featureRows [][]string, features *catalogs.ModelFeatures) [][]string {
	// Check for vision capability
	for _, modality := range features.Modalities.Input {
		if modality == "image" {
			featureRows = append(featureRows, []string{"Vision", emoji.Success + " Supported"})
			break
		}
	}

	// Check for audio capabilities
	hasAudioInput := false
	hasAudioOutput := false
	for _, modality := range features.Modalities.Input {
		if modality == "audio" {
			hasAudioInput = true
			break
		}
	}
	for _, modality := range features.Modalities.Output {
		if modality == "audio" {
			hasAudioOutput = true
			break
		}
	}
	if hasAudioInput {
		featureRows = append(featureRows, []string{"Audio Input", emoji.Success + " Supported"})
	}
	if hasAudioOutput {
		featureRows = append(featureRows, []string{"Audio Output", emoji.Success + " Supported"})
	}

	return featureRows
}

func printArchitectureInfo(model *catalogs.Model, formatter format.Formatter) {
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return
	}

	var archRows [][]string
	if model.Metadata.Architecture.ParameterCount != "" {
		archRows = append(archRows, []string{"Size", model.Metadata.Architecture.ParameterCount})
	}
	if model.Metadata.Architecture.Tokenizer != "" {
		archRows = append(archRows, []string{"Tokenizer", model.Metadata.Architecture.Tokenizer.String()})
	}

	if len(archRows) > 0 {
		archTable := format.Data{
			Headers: []string{"Property", "Value"},
			Rows:    archRows,
		}
		fmt.Println("Architecture:")
		_ = formatter.Format(os.Stdout, archTable)
		fmt.Println()
	}
}
