// Package table provides common table formatting utilities for CLI commands.
package table

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Data represents table formatting data to avoid import cycles.
type Data struct {
	Headers []string
	Rows    [][]string
}

// ModelsToTableData converts models to table format.
func ModelsToTableData(models []*catalogs.Model, showDetails bool) Data {
	headers := []string{"ID", "Name", "Context", "Output", "Input Price", "Output Price"}
	if showDetails {
		headers = append(headers, "Features", "Authors", "Description")
	}

	rows := make([][]string, 0, len(models))
	for _, model := range models {
		row := []string{
			model.ID,
			model.Name,
			FormatTokens(model.Limits),
			FormatOutput(model.Limits),
			FormatPrice(model.Pricing, true),
			FormatPrice(model.Pricing, false),
		}

		if showDetails {
			features := BuildFeaturesString(model)
			if features == "" {
				features = "-"
			}

			authors := BuildAuthorsString(model.Authors)
			if authors == "" {
				authors = "-"
			}

			description := model.Description
			if len(description) > 80 {
				description = description[:77] + "..."
			}
			if description == "" {
				description = "-"
			}

			row = append(row, features, authors, description)
		}

		rows = append(rows, row)
	}

	return Data{
		Headers: headers,
		Rows:    rows,
	}
}

// FormatTokens formats context window information.
func FormatTokens(limits *catalogs.ModelLimits) string {
	if limits == nil {
		return "-"
	}
	if limits.ContextWindow > 0 {
		return FormatNumber(limits.ContextWindow)
	}
	return "-"
}

// FormatOutput formats output token limit information.
func FormatOutput(limits *catalogs.ModelLimits) string {
	if limits == nil {
		return "-"
	}
	if limits.OutputTokens > 0 {
		return FormatNumber(limits.OutputTokens)
	}
	return "-"
}

// FormatPrice formats pricing information.
func FormatPrice(pricing *catalogs.ModelPricing, input bool) string {
	if pricing == nil || pricing.Tokens == nil {
		return "-"
	}

	var cost float64
	if input {
		if pricing.Tokens.Input != nil {
			cost = pricing.Tokens.Input.Per1M
		}
	} else {
		if pricing.Tokens.Output != nil {
			cost = pricing.Tokens.Output.Per1M
		}
	}

	if cost == 0 {
		return "-"
	}

	return fmt.Sprintf("$%.6f", cost)
}

// FormatNumber formats large numbers with comma separators.
func FormatNumber(n int64) string {
	str := strconv.FormatInt(n, 10)
	if len(str) <= 3 {
		return str
	}

	// Add commas every 3 digits
	result := ""
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(r)
	}
	return result
}

// BuildFeaturesString creates a comma-separated list of model features.
func BuildFeaturesString(model *catalogs.Model) string {
	var features []string

	if model.Features != nil {
		// Check for vision in modalities
		for _, modality := range model.Features.Modalities.Input {
			if modality == "image" {
				features = append(features, "vision")
				break
			}
		}

		// Check for audio in modalities
		for _, modality := range model.Features.Modalities.Input {
			if modality == "audio" {
				features = append(features, "audio_input")
				break
			}
		}
		for _, modality := range model.Features.Modalities.Output {
			if modality == "audio" {
				features = append(features, "audio_output")
				break
			}
		}

		if model.Features.ToolCalls {
			features = append(features, "tool_calls")
		}

		if model.Features.WebSearch {
			features = append(features, "web_search")
		}

		if model.Features.Reasoning {
			features = append(features, "reasoning")
		}
	}

	return strings.Join(features, ", ")
}

// BuildAuthorsString creates a comma-separated list of author names.
func BuildAuthorsString(authors []catalogs.Author) string {
	if len(authors) == 0 {
		return ""
	}

	names := make([]string, len(authors))
	for i, author := range authors {
		names[i] = author.Name
	}
	return strings.Join(names, ", ")
}

// ProvidersToTableData converts providers to table format.
func ProvidersToTableData(providers []*catalogs.Provider, showKeys bool) Data {
	headers := []string{"ID", "NAME", "LOCATION", "STATUS"}

	rows := make([][]string, 0, len(providers))
	for _, provider := range providers {
		status := "✓"
		if showKeys && provider.APIKey != nil {
			if provider.IsAPIKeyRequired() {
				// Check if key is configured
				if os.Getenv(provider.APIKey.Name) == "" {
					status = "✗"
				}
			}
		}

		location := ""
		if provider.Headquarters != nil {
			location = *provider.Headquarters
		}

		row := []string{
			string(provider.ID),
			provider.Name,
			location,
			status,
		}
		rows = append(rows, row)
	}

	return Data{
		Headers: headers,
		Rows:    rows,
	}
}

// AuthorsToTableData converts authors to table format.
func AuthorsToTableData(authors []*catalogs.Author) Data {
	headers := []string{"ID", "NAME", "MODELS", "WEBSITE"}

	rows := make([][]string, 0, len(authors))
	for _, author := range authors {
		website := ""
		if author.Website != nil {
			website = *author.Website
		}

		row := []string{
			string(author.ID),
			author.Name,
			fmt.Sprintf("%d", len(author.Models)),
			website,
		}
		rows = append(rows, row)
	}

	return Data{
		Headers: headers,
		Rows:    rows,
	}
}
