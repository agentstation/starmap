// Package list provides shared utilities for list subcommands.
package list

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/cmdutil"
)

// getGlobalFlags returns the global flags using the same approach as other commands.
// We'll pass these as parameters instead of accessing a global variable.
func getGlobalFlags() *cmdutil.GlobalFlags {
	// Return defaults for now - this will be passed from the calling commands
	return &cmdutil.GlobalFlags{
		Output: "",
		Quiet:  false,
	}
}

// getResourceFlags extracts resource flags from a command.
func getResourceFlags(cmd *cobra.Command) *cmdutil.ResourceFlags {
	provider, _ := cmd.Flags().GetString("provider")
	author, _ := cmd.Flags().GetString("author")
	search, _ := cmd.Flags().GetString("search")
	limit, _ := cmd.Flags().GetInt("limit")

	return &cmdutil.ResourceFlags{
		Provider: provider,
		Author:   author,
		Search:   search,
		Limit:    limit,
	}
}
