package globals

import "github.com/spf13/cobra"

// Flags holds global common flags across all commands.
type Flags struct {
	Output  string
	Quiet   bool
	Verbose bool
	NoColor bool
}

// AddFlags adds common flags to the root command.
func AddFlags(cmd *cobra.Command) *Flags {
	flags := &Flags{}

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
