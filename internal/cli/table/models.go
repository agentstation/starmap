// Package table provides common table formatting utilities for CLI commands.
package table

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cli/emoji"
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
	var result strings.Builder
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(r)
	}
	return result.String()
}

// BuildFeaturesString creates a comma-separated list of model features.
func BuildFeaturesString(model *catalogs.Model) string {
	var features []string

	if model.Features != nil {
		// Check for vision in modalities
		if slices.Contains(model.Features.Modalities.Input, "image") {
			features = append(features, "vision")
		}

		// Check for audio in modalities
		if slices.Contains(model.Features.Modalities.Input, "audio") {
			features = append(features, "audio_input")
		}
		if slices.Contains(model.Features.Modalities.Output, "audio") {
			features = append(features, "audio_output")
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
	headers := []string{"NAME", "ID", "SOURCE", "API TYPE", "AUTH", "ENV", "MODELS", "STATUS"}

	rows := make([][]string, 0, len(providers))
	for _, provider := range providers {
		statuses := checker.CheckSources(context.Background(), provider)
		bySource := make(map[string]auth.SourceStatus, len(statuses))
		for _, status := range statuses {
			bySource[status.SourceID] = status
		}
		if provider.Catalog == nil || len(provider.Catalog.Sources) == 0 {
			state := auth.StateInvalid
			if !supportedMap[string(provider.ID)] {
				state = auth.StateUnsupported
			}
			statusIcon, statusText := getStatusDisplay(state)
			rows = append(rows, []string{provider.Name, string(provider.ID), "-", "-", "-", "-", fmt.Sprintf("%d", len(provider.Models)), statusIcon + " " + statusText})
			continue
		}
		for _, source := range provider.Catalog.Sources {
			status := bySource[source.ID]
			if !supportedMap[string(provider.ID)] {
				status.State = auth.StateUnsupported
			}
			statusIcon, statusText := getStatusDisplay(status.State)
			authMethods := "none"
			if source.Auth.Mode == catalogs.ProviderAuthModeOptional {
				authMethods = "optional"
			} else if len(source.Auth.Methods) > 0 {
				methods := make([]string, len(source.Auth.Methods))
				for index, method := range source.Auth.Methods {
					methods[index] = string(method)
				}
				authMethods = strings.Join(methods, ",")
			}
			environment := "-"
			if len(status.Environment) > 0 {
				environment = strings.Join(status.Environment, ",")
			}
			rows = append(rows, []string{
				provider.Name, string(provider.ID), source.ID, string(source.Endpoint.Type),
				authMethods, environment, fmt.Sprintf("%d", len(provider.Models)), statusIcon + " " + statusText,
			})
		}
	}

	return Data{
		Headers: headers,
		Rows:    rows,
		ColumnAlignment: []Align{
			AlignDefault, // NAME
			AlignDefault, // ID
			AlignDefault, // SOURCE
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
	case auth.StateReady:
		return emoji.Success, "Ready"
	case auth.StateUnavailable:
		return emoji.Error, "Unavailable"
	case auth.StateInvalid:
		return emoji.Warning, "Invalid"
	case auth.StateUnauthenticated:
		return emoji.Optional, "Unauthenticated"
	case auth.StateUnsupported:
		return emoji.Unsupported, "Unsupported"
	default:
		return emoji.Unknown, "Unknown"
	}
}
