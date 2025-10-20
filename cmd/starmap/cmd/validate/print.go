package validate

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap/internal/cmd/format"
)

// displayValidationTable shows validation results in a table format.
func displayValidationTable(results []ValidationResult, verbose bool) {
	if len(results) == 0 {
		return
	}

	formatter := format.NewFormatter(format.FormatTable)

	headers := []string{"Component", "Status", "Issues"}
	if verbose {
		headers = append(headers, "Details")
	}

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		row := []string{
			result.Component,
			result.Status,
			result.Issues,
		}
		if verbose {
			details := result.Details
			if len(details) > 80 {
				details = details[:77] + "..."
			}
			row = append(row, details)
		}
		rows = append(rows, row)
	}

	tableData := format.Data{
		Headers: headers,
		Rows:    rows,
	}

	fmt.Println("Catalog Validation Results:")
	_ = formatter.Format(os.Stdout, tableData)
	fmt.Println()
}
