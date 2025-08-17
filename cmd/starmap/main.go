package main

import (
	"github.com/agentstation/starmap/cmd/starmap/cmd"

	// Import internal catalog packages to register their constructors
	_ "github.com/agentstation/starmap/internal/catalogs/embedded"
	_ "github.com/agentstation/starmap/internal/catalogs/files"
	_ "github.com/agentstation/starmap/internal/catalogs/memory"
)

func main() {
	cmd.Execute()
}
