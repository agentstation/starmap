package authors

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// printAuthorDetails prints detailed author information.
func printAuthorDetails(author *catalogs.Author) {
	formatter := format.NewFormatter(format.FormatTable)

	fmt.Printf("Author: %s\n\n", author.ID)

	// Basic info
	basicRows := [][]string{
		{"Author ID", string(author.ID)},
		{"Name", author.Name},
	}

	if author.Description != nil && *author.Description != "" {
		basicRows = append(basicRows, []string{"Description", *author.Description})
	}
	if author.Website != nil && *author.Website != "" {
		basicRows = append(basicRows, []string{"Website", *author.Website})
	}

	basicTable := format.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()
}
