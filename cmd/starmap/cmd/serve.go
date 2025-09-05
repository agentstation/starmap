package cmd

import (
	"github.com/agentstation/starmap/cmd/starmap/cmd/serve"
)

func init() {
	rootCmd.AddCommand(serve.NewCommand())
}
