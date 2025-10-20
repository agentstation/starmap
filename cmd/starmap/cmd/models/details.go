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

// printModelDetails prints detailed model information in a single comprehensive table.
func printModelDetails(model *catalogs.Model, provider catalogs.Provider) {
	formatter := format.NewFormatter(format.FormatTable)

	fmt.Printf("Model: %s\n\n", model.ID)

	var rows [][]string

	// Build table rows by section
	rows = addIdentityRows(rows, model, provider)
	rows = addLimitRows(rows, model)
	rows = addPricingRows(rows, model)
	rows = addMetadataRows(rows, model)
	rows = addFeatureRows(rows, model)
	rows = addArchitectureRows(rows, model)
	rows = addDescriptionRow(rows, model)

	detailsTable := format.Data{
		Headers: []string{"Property", "Value"},
		Rows:    rows,
	}

	_ = formatter.Format(os.Stdout, detailsTable)
}

// addIdentityRows adds core identity information to the table.
func addIdentityRows(rows [][]string, model *catalogs.Model, provider catalogs.Provider) [][]string {
	rows = append(rows, []string{"Model ID", model.ID})
	rows = append(rows, []string{"Name", model.Name})
	rows = append(rows, []string{"Provider", fmt.Sprintf("%s (%s)", provider.Name, provider.ID)})

	if len(model.Authors) > 0 {
		authorNames := make([]string, len(model.Authors))
		for i, author := range model.Authors {
			authorNames[i] = author.Name
		}
		rows = append(rows, []string{"Authors", strings.Join(authorNames, ", ")})
	} else {
		rows = append(rows, []string{"Authors", "Unknown"})
	}

	return rows
}

// addLimitRows adds limit information to the table.
func addLimitRows(rows [][]string, model *catalogs.Model) [][]string {
	if model.Limits == nil {
		return rows
	}

	if model.Limits.ContextWindow > 0 {
		rows = append(rows, []string{"Context Window", fmt.Sprintf("%s tokens", table.FormatNumber(model.Limits.ContextWindow))})
	}
	if model.Limits.OutputTokens > 0 {
		rows = append(rows, []string{"Max Output", fmt.Sprintf("%s tokens", table.FormatNumber(model.Limits.OutputTokens))})
	}

	return rows
}

// addPricingRows adds pricing information to the table.
func addPricingRows(rows [][]string, model *catalogs.Model) [][]string {
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		return rows
	}

	if model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M > 0 {
		rows = append(rows, []string{"Input Price", fmt.Sprintf("$%.2f per 1M tokens", model.Pricing.Tokens.Input.Per1M)})
	}
	if model.Pricing.Tokens.Output != nil && model.Pricing.Tokens.Output.Per1M > 0 {
		rows = append(rows, []string{"Output Price", fmt.Sprintf("$%.2f per 1M tokens", model.Pricing.Tokens.Output.Per1M)})
	}
	if model.Pricing.Tokens.Reasoning != nil && model.Pricing.Tokens.Reasoning.Per1M > 0 {
		rows = append(rows, []string{"Reasoning Price", fmt.Sprintf("$%.2f per 1M tokens", model.Pricing.Tokens.Reasoning.Per1M)})
	}

	return rows
}

// addMetadataRows adds metadata information to the table.
func addMetadataRows(rows [][]string, model *catalogs.Model) [][]string {
	if model.Metadata == nil {
		return rows
	}

	if !model.Metadata.ReleaseDate.IsZero() {
		rows = append(rows, []string{"Released", model.Metadata.ReleaseDate.Format("2006-01")})
	}
	rows = append(rows, []string{"Open Weights", formatBool(model.Metadata.OpenWeights)})

	return rows
}

// addFeatureRows adds feature information to the table.
func addFeatureRows(rows [][]string, model *catalogs.Model) [][]string {
	if model.Features == nil {
		return rows
	}

	// Check for vision capability
	for _, modality := range model.Features.Modalities.Input {
		if modality == "image" {
			rows = append(rows, []string{"Vision", emoji.Success + " Supported"})
			break
		}
	}

	// Check for audio capabilities
	rows = addAudioFeatures(rows, model.Features)

	// Other key features
	if model.Features.ToolCalls {
		rows = append(rows, []string{"Tool Calls", emoji.Success + " Supported"})
	}
	if model.Features.WebSearch {
		rows = append(rows, []string{"Web Search", emoji.Success + " Supported"})
	}
	if model.Features.Reasoning {
		rows = append(rows, []string{"Reasoning", emoji.Success + " Supported"})
	}

	return rows
}

// addAudioFeatures adds audio feature information to the table.
func addAudioFeatures(rows [][]string, features *catalogs.ModelFeatures) [][]string {
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
		rows = append(rows, []string{"Audio Input", emoji.Success + " Supported"})
	}
	if hasAudioOutput {
		rows = append(rows, []string{"Audio Output", emoji.Success + " Supported"})
	}

	return rows
}

// addArchitectureRows adds architecture information to the table.
func addArchitectureRows(rows [][]string, model *catalogs.Model) [][]string {
	if model.Metadata == nil || model.Metadata.Architecture == nil {
		return rows
	}

	if model.Metadata.Architecture.ParameterCount != "" {
		rows = append(rows, []string{"Parameter Count", model.Metadata.Architecture.ParameterCount})
	}
	if model.Metadata.Architecture.Tokenizer != "" {
		rows = append(rows, []string{"Tokenizer", model.Metadata.Architecture.Tokenizer.String()})
	}

	return rows
}

// addDescriptionRow adds the description to the table.
func addDescriptionRow(rows [][]string, model *catalogs.Model) [][]string {
	if model.Description == "" {
		return rows
	}

	description := model.Description
	if len(description) > 100 {
		description = description[:97] + "..."
	}
	rows = append(rows, []string{"Description", description})

	return rows
}

// formatBool formats a boolean value as Yes/No.
func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
