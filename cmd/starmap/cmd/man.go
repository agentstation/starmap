package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manCmd represents the man command
var manCmd = &cobra.Command{
	Use:    "man",
	Short:  "Generate man page",
	Long:   `Generate man page for starmap CLI tool.`,
	Hidden: true, // Hide from help output since it's mainly for internal use
	RunE: func(cmd *cobra.Command, args []string) error {
		header := &doc.GenManHeader{
			Title:   "STARMAP",
			Section: "1",
			Source:  "starmap",
			Manual:  "starmap Manual",
		}
		return doc.GenMan(cmd.Root(), header, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
