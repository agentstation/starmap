package main

import (
	"github.com/agentstation/starmap/cmd/starmap/cmd"
)

// Version information populated by goreleaser
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cmd.Execute(version, commit, date, builtBy)
}
