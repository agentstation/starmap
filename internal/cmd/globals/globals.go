// Package globals provides shared flag structures and utilities for CLI commands.
package globals

import "github.com/spf13/cobra"

// Flags holds global common flags across all commands.
type Flags struct {
	Format  string
	Quiet   bool
	Verbose bool
	NoColor bool
}

// AddFlags adds common flags to the root command.
func AddFlags(cmd *cobra.Command) *Flags {
	flags := &Flags{}

	cmd.PersistentFlags().StringVarP(&flags.Format, "format", "f", "",
		"Output format: table, json, yaml, wide")
	cmd.PersistentFlags().BoolVarP(&flags.Quiet, "quiet", "q", false,
		"Minimal output")
	cmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false,
		"Verbose output")
	cmd.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false,
		"Disable colored output")

	return flags
}

// Parse extracts global flags from the command hierarchy.
// This is useful for subcommands that need to access global flags when
// they weren't passed the flags struct directly.
func Parse(cmd *cobra.Command) (*Flags, error) {
	// Walk up the command hierarchy to find persistent flags
	root := cmd
	for root.Parent() != nil {
		root = root.Parent()
	}

	format, _ := root.PersistentFlags().GetString("format")
	quiet, _ := root.PersistentFlags().GetBool("quiet")
	verbose, _ := root.PersistentFlags().GetBool("verbose")
	noColor, _ := root.PersistentFlags().GetBool("no-color")

	return &Flags{
		Format:  format,
		Quiet:   quiet,
		Verbose: verbose,
		NoColor: noColor,
	}, nil
}
