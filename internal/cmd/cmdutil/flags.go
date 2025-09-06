// Package cmdutil provides shared flags and configuration utilities for starmap commands.
package cmdutil

import (
	"github.com/spf13/cobra"
)

// GlobalFlags holds common flags across all commands.
type GlobalFlags struct {
	Output  string
	Quiet   bool
	Verbose bool
	NoColor bool
}

// AddGlobalFlags adds common flags to the root command.
func AddGlobalFlags(cmd *cobra.Command) *GlobalFlags {
	flags := &GlobalFlags{}

	cmd.PersistentFlags().StringVarP(&flags.Output, "output", "o", "",
		"Output format: table, json, yaml, wide")
	cmd.PersistentFlags().BoolVarP(&flags.Quiet, "quiet", "q", false,
		"Minimal output")
	cmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false,
		"Verbose output")
	cmd.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false,
		"Disable colored output")

	return flags
}

// ResourceFlags holds flags for resource-specific operations.
type ResourceFlags struct {
	Provider string
	Author   string
	Limit    int
	Search   string
	Filter   []string
	All      bool
}

// AddResourceFlags adds resource-specific flags to a command.
func AddResourceFlags(cmd *cobra.Command) *ResourceFlags {
	flags := &ResourceFlags{}

	cmd.Flags().StringVarP(&flags.Provider, "provider", "p", "",
		"Filter by provider")
	cmd.Flags().StringVar(&flags.Author, "author", "",
		"Filter by author")
	cmd.Flags().IntVarP(&flags.Limit, "limit", "l", 0,
		"Limit number of results")
	cmd.Flags().StringVar(&flags.Search, "search", "",
		"Search term to filter results")
	cmd.Flags().StringSliceVar(&flags.Filter, "filter", nil,
		"Filter expressions (e.g., 'context>100000')")
	cmd.Flags().BoolVar(&flags.All, "all", false,
		"Include all results (no filtering)")

	return flags
}
