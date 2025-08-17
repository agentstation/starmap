package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all models from embedded catalog",
	Long: `List displays all AI models available in the embedded catalog.
	
The embedded catalog is compiled into the binary at build time and
provides offline access to model information including capabilities,
limits, and features.`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	catalog, err := starmap.NewEmbeddedCatalog()
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	// Get all models from the catalog
	models := catalog.Models().List()
	if len(models) == 0 {
		fmt.Println("No models found in catalog")
		return nil
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	fmt.Printf("Found %d models in catalog:\n\n", len(models))
	for _, model := range models {
		fmt.Printf("• %s - %s\n", model.ID, model.Name)
		if model.Description != "" {
			fmt.Printf("  %s\n", model.Description)
		}
		if len(model.Authors) > 0 {
			authors := make([]string, 0, len(model.Authors))
			for _, a := range model.Authors {
				authors = append(authors, a.Name)
			}
			fmt.Printf("  Authors: %s\n", strings.Join(authors, ", "))
		}
		if model.Limits != nil {
			if model.Limits.ContextWindow > 0 {
				fmt.Printf("  Context: %d tokens", model.Limits.ContextWindow)
				if model.Limits.OutputTokens > 0 {
					fmt.Printf(", Output: %d tokens", model.Limits.OutputTokens)
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}

	return nil
}
