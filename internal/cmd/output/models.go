// Package output provides common output formatting utilities for CLI commands.
package output

import (
	"os"

	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// FormatModels handles the common pattern of formatting models for output.
// This encapsulates the switch logic for different output formats.
func FormatModels(models []*catalogs.Model, showDetails bool, globalFlags *globals.Flags) error {
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

// FormatProviders handles the common pattern of formatting providers for output.
func FormatProviders(providers []*catalogs.Provider, showKeys bool, globalFlags *globals.Flags) error {
	formatter := NewFormatter(Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		outputData = table.ProvidersToTableData(providers, showKeys)
	default:
		outputData = providers
	}

	return formatter.Format(os.Stdout, outputData)
}

// FormatAuthors handles the common pattern of formatting authors for output.
func FormatAuthors(authors []*catalogs.Author, globalFlags *globals.Flags) error {
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

// FormatAny handles the common pattern of formatting any data type for output.
// This is useful for commands with custom data structures.
func FormatAny(data any, globalFlags *globals.Flags) error {
	formatter := NewFormatter(Format(globalFlags.Output))
	return formatter.Format(os.Stdout, data)
}
