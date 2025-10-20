package providers

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// printProviderDetails prints detailed provider information.
func printProviderDetails(provider *catalogs.Provider) {
	formatter := format.NewFormatter(format.FormatTable)

	fmt.Printf("Provider: %s\n\n", provider.ID)

	// Basic info
	basicRows := [][]string{
		{"Provider ID", string(provider.ID)},
		{"Name", provider.Name},
	}

	if provider.Headquarters != nil && *provider.Headquarters != "" {
		basicRows = append(basicRows, []string{"Headquarters", *provider.Headquarters})
	}

	if provider.StatusPageURL != nil && *provider.StatusPageURL != "" {
		basicRows = append(basicRows, []string{"Status Page", *provider.StatusPageURL})
	}

	basicTable := format.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()

	// Model count
	fmt.Printf("Models: %d\n", len(provider.Models))
}
