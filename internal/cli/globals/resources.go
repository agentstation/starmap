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
// The command must have had AddResourceFlags called on it, otherwise this will panic.
func ParseResources(cmd *cobra.Command) *ResourceFlags {
	provider := mustGetString(cmd, "provider")
	author := mustGetString(cmd, "author")
	search := mustGetString(cmd, "search")
	limit := mustGetInt(cmd, "limit")

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

// mustGetString retrieves a string flag value or panics if the flag doesn't exist.
func mustGetString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}

// mustGetInt retrieves an integer flag value or panics if the flag doesn't exist.
func mustGetInt(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}
