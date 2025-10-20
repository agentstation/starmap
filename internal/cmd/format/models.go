// Package format provides common output formatting utilities for CLI commands.
package format

import (
	"os"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Models handles the common pattern of formatting models for output.
// This encapsulates the switch logic for different output formats.
func Models(models []*catalogs.Model, showDetails bool, globalFlags *globals.Flags) error {
	formatter := NewFormatter(Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		outputData = table.ModelsToTableData(models, showDetails)
	default:
		outputData = models
	}

	return formatter.Format(os.Stdout, outputData)
}

// Providers handles the common pattern of formatting providers for output.
func Providers(providers []*catalogs.Provider, checker *auth.Checker, supportedMap map[string]bool, globalFlags *globals.Flags) error {
	formatter := NewFormatter(Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		outputData = table.ProvidersToTableData(providers, checker, supportedMap)
	default:
		outputData = providers
	}

	return formatter.Format(os.Stdout, outputData)
}

// Authors handles the common pattern of formatting authors for output.
func Authors(authors []*catalogs.Author, globalFlags *globals.Flags) error {
	formatter := NewFormatter(Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		outputData = table.AuthorsToTableData(authors)
	default:
		outputData = authors
	}

	return formatter.Format(os.Stdout, outputData)
}

// Any handles the common pattern of formatting any data type for output.
// This is useful for commands with custom data structures.
func Any(data any, globalFlags *globals.Flags) error {
	formatter := NewFormatter(Format(globalFlags.Output))
	return formatter.Format(os.Stdout, data)
}
