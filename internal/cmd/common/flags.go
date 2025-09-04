package common

import (
	"github.com/spf13/cobra"
)

// GlobalFlags holds common flags across all commands
type GlobalFlags struct {
	Output   string
	Quiet    bool
	Verbose  bool
	NoColor  bool
}

// AddGlobalFlags adds common flags to the root command
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

// ResourceFlags holds flags for resource-specific operations
type ResourceFlags struct {
	Provider   string
	Author     string
	Limit      int
	Search     string
	Filter     []string
	All        bool
}

// AddResourceFlags adds resource-specific flags to a command
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

// UpdateFlags holds flags for update command
type UpdateFlags struct {
	Provider    string
	Source      string
	DryRun      bool
	Force       bool
	AutoApprove bool
	Output      string
	Input       string
	Cleanup     bool
	Reformat    bool
}

// AddUpdateFlags adds update-specific flags to the update command
func AddUpdateFlags(cmd *cobra.Command) *UpdateFlags {
	flags := &UpdateFlags{}
	
	cmd.Flags().StringVarP(&flags.Provider, "provider", "p", "", 
		"Update specific provider only")
	cmd.Flags().StringVar(&flags.Source, "source", "", 
		"Update from specific source (provider-api, models.dev)")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, 
		"Preview changes without applying them")
	cmd.Flags().BoolVarP(&flags.Force, "force", "f", false, 
		"Force fresh update (delete and recreate)")
	cmd.Flags().BoolVarP(&flags.AutoApprove, "yes", "y", false, 
		"Auto-approve changes without confirmation")
	cmd.Flags().StringVar(&flags.Output, "output", "", 
		"Save updated catalog to directory")
	cmd.Flags().StringVar(&flags.Input, "input", "", 
		"Load catalog from directory instead of embedded")
	cmd.Flags().BoolVar(&flags.Cleanup, "cleanup", false, 
		"Remove temporary models.dev repository after update")
	cmd.Flags().BoolVar(&flags.Reformat, "reformat", false, 
		"Reformat catalog files even without changes")
	
	return flags
}

// FetchFlags holds flags for fetch command
type FetchFlags struct {
	Provider string
	All      bool
	Timeout  int
}

// AddFetchFlags adds fetch-specific flags
func AddFetchFlags(cmd *cobra.Command) *FetchFlags {
	flags := &FetchFlags{}
	
	cmd.Flags().StringVarP(&flags.Provider, "provider", "p", "", 
		"Provider to fetch from")
	cmd.Flags().BoolVar(&flags.All, "all", false, 
		"Fetch from all configured providers")
	cmd.Flags().IntVar(&flags.Timeout, "timeout", 30, 
		"Timeout in seconds for API calls")
	
	return flags
}

// ExportFlags holds flags for export command
type ExportFlags struct {
	Format   string
	Provider string
	Output   string
	Pretty   bool
}

// AddExportFlags adds export-specific flags
func AddExportFlags(cmd *cobra.Command) *ExportFlags {
	flags := &ExportFlags{}
	
	cmd.Flags().StringVarP(&flags.Format, "format", "f", "openai", 
		"Export format: openai, openrouter")
	cmd.Flags().StringVarP(&flags.Provider, "provider", "p", "", 
		"Provider to export (optional, exports all if not specified)")
	cmd.Flags().StringVar(&flags.Output, "output", "", 
		"Output file (default: stdout)")
	cmd.Flags().BoolVar(&flags.Pretty, "pretty", true, 
		"Pretty print JSON output")
	
	return flags
}