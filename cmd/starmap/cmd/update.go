package cmd

import (
	"github.com/agentstation/starmap/cmd/starmap/cmd/update"
)

func init() {
	rootCmd.AddCommand(update.NewCommand(globalFlags))
}
