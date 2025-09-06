package globals

import "github.com/spf13/cobra"

// ResourceFlags holds flags for resource-specific operations.
type ResourceFlags struct {
	Provider string
	Author   string
	Limit    int
	Search   string
	Filter   []string
	All      bool
}

// ParseResources extracts resource flags from a command.
func ParseResources(cmd *cobra.Command) *ResourceFlags {
	provider, _ := cmd.Flags().GetString("provider")
	author, _ := cmd.Flags().GetString("author")
	search, _ := cmd.Flags().GetString("search")
	limit, _ := cmd.Flags().GetInt("limit")

	return &ResourceFlags{
		Provider: provider,
		Author:   author,
		Search:   search,
		Limit:    limit,
	}
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
