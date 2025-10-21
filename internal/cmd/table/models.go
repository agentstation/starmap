// Package table provides common table formatting utilities for CLI commands.
package table

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Align represents column alignment in tables.
type Align int

const (
	// AlignDefault uses the default alignment (skip).
	AlignDefault Align = iota
	// AlignLeft aligns content to the left.
	AlignLeft
	// AlignCenter centers content.
	AlignCenter
	// AlignRight aligns content to the right.
	AlignRight
)

// Data represents table formatting data to avoid import cycles.
type Data struct {
	Headers         []string
	Rows            [][]string
	ColumnAlignment []Align // Optional: column alignment
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
func ProvidersToTableData(providers []*catalogs.Provider, checker *auth.Checker, supportedMap map[string]bool) Data {
	headers := []string{"NAME", "ID", "LOCATION", "API TYPE", "ENV KEY", "KEY", "MODELS", "STATUS"}

	rows := make([][]string, 0, len(providers))
	for _, provider := range providers {
		// Check authentication status
		authStatus := checker.CheckProvider(provider, supportedMap)
		statusIcon, statusText := getStatusDisplay(authStatus.State)

		location := ""
		if provider.Headquarters != nil {
			location = *provider.Headquarters
		}

		row := []string{
			provider.Name,
			string(provider.ID),
			location,
			getEndpointType(provider),
			getKeyVariable(provider, authStatus),
			getKeyPreview(provider, authStatus),
			fmt.Sprintf("%d", len(provider.Models)),
			statusIcon + " " + statusText,
		}
		rows = append(rows, row)
	}

	return Data{
		Headers: headers,
		Rows:    rows,
		ColumnAlignment: []Align{
			AlignDefault, // NAME
			AlignDefault, // ID
			AlignDefault, // LOCATION
			AlignDefault, // TYPE
			AlignDefault, // ENV KEY
			AlignDefault, // KEY
			AlignCenter,  // MODELS (centered)
			AlignDefault, // STATUS
		},
	}
}

// AuthorsToTableData converts authors to table format.
func AuthorsToTableData(authors []*catalogs.Author) Data {
	headers := []string{"ID", "NAME", "WEBSITE", "MODELS"}

	rows := make([][]string, 0, len(authors))
	for _, author := range authors {
		website := ""
		if author.Website != nil {
			website = *author.Website
		}

		row := []string{
			string(author.ID),
			author.Name,
			website,
			fmt.Sprintf("%d", len(author.Models)),
		}
		rows = append(rows, row)
	}

	return Data{
		Headers:         headers,
		Rows:            rows,
		ColumnAlignment: []Align{AlignDefault, AlignDefault, AlignDefault, AlignCenter}, // Center the MODELS column
	}
}

// getStatusDisplay returns icon and text for a status state.
func getStatusDisplay(state auth.State) (string, string) {
	switch state {
	case auth.StateConfigured:
		return emoji.Success, "Configured"
	case auth.StateMissing:
		return emoji.Error, "Missing"
	case auth.StateInvalid:
		return emoji.Warning, "Invalid"
	case auth.StateOptional:
		return emoji.Optional, "Optional"
	case auth.StateUnsupported:
		return emoji.Unsupported, "Unsupported"
	default:
		return emoji.Unknown, "Unknown"
	}
}

// getKeyVariable returns the key variable name or special message for display.
func getKeyVariable(provider *catalogs.Provider, status *auth.Status) string {
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return "(gcloud auth)"
	}

	if provider.APIKey != nil {
		return provider.APIKey.Name
	}

	if status.State == auth.StateUnsupported {
		return "(no implementation)"
	}

	return "(no key required)"
}

// getKeyPreview returns a masked preview of the API key for display in tables.
func getKeyPreview(provider *catalogs.Provider, status *auth.Status) string {
	// Google Cloud providers use gcloud auth, not API keys
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return "(gcloud auth)"
	}

	// Unsupported providers
	if status.State == auth.StateUnsupported {
		return "(n/a)"
	}

	// API key providers
	if provider.APIKey != nil {
		envValue := os.Getenv(provider.APIKey.Name)
		if envValue != "" {
			return maskAPIKey(envValue)
		}
		return "(not set)"
	}

	return "-"
}

// maskAPIKey masks an API key for safe display, showing only first 8 and last 4 characters.
func maskAPIKey(key string) string {
	if key == "" {
		return "(empty)"
	}

	// For very short keys, just show asterisks
	if len(key) <= 12 {
		return strings.Repeat("*", len(key))
	}

	// Show first 8 characters, mask middle, show last 4
	prefix := key[:8]
	suffix := key[len(key)-4:]
	masked := strings.Repeat("*", 24) // 24 asterisks in the middle

	return prefix + masked + suffix
}

// getEndpointType returns the endpoint type for a provider.
func getEndpointType(provider *catalogs.Provider) string {
	if provider.Catalog != nil && provider.Catalog.Endpoint.Type != "" {
		return string(provider.Catalog.Endpoint.Type)
	}
	return "-"
}
