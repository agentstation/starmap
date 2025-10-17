// Package output provides formatters for command output.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mattn/go-isatty"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/agentstation/starmap/internal/cmd/table"
)

// Format types for output.
type Format string

const (
	// FormatTable represents table output format.
	FormatTable Format = "table"
	// FormatJSON represents JSON output format.
	FormatJSON Format = "json"
	// FormatYAML represents YAML output format.
	FormatYAML Format = "yaml"
	// FormatWide represents wide table output format.
	FormatWide Format = "wide"
)

// Formatter interface for all output types.
type Formatter interface {
	Format(w io.Writer, data any) error
}

// FormatterFunc allows functions to implement Formatter.
type FormatterFunc func(io.Writer, any) error

// Format implements the Formatter interface.
func (f FormatterFunc) Format(w io.Writer, data any) error {
	return f(w, data)
}

// NewFormatter creates appropriate formatter based on format.
func NewFormatter(format Format) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{Indent: "  "}
	case FormatYAML:
		return &YAMLFormatter{}
	case FormatTable, FormatWide:
		return &TableFormatter{Wide: format == FormatWide}
	default:
		return &TableFormatter{}
	}
}

// JSONFormatter outputs JSON format.
type JSONFormatter struct {
	Indent string
}

// Format implements the Formatter interface for JSON output.
func (f *JSONFormatter) Format(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	if f.Indent != "" {
		encoder.SetIndent("", f.Indent)
	}
	return encoder.Encode(data)
}

// YAMLFormatter outputs YAML format.
type YAMLFormatter struct{}

// Format outputs data in YAML format.
func (f *YAMLFormatter) Format(w io.Writer, data any) error {
	yamlData, err := yaml.MarshalWithOptions(data,
		yaml.Indent(2),
		yaml.IndentSequence(false),
	)
	if err != nil {
		return err
	}
	_, err = w.Write(yamlData)
	return err
}

// TableFormatter outputs table format.
type TableFormatter struct {
	Wide bool
}

// Format outputs data in table format.
func (f *TableFormatter) Format(w io.Writer, data any) error {
	// Type switch to handle different data types
	switch v := data.(type) {
	case Data:
		return f.formatTable(w, v)
	default:
		// Try to convert structs/slices to table format using reflection
		if tableData := f.convertToTableData(data); tableData != nil {
			return f.formatTable(w, *tableData)
		}

		// Fall back to JSON for non-table data
		jsonFormatter := &JSONFormatter{Indent: "  "}
		return jsonFormatter.Format(w, data)
	}
}

func (f *TableFormatter) formatTable(w io.Writer, data Data) error {
	// Use tablewriter for proper table formatting
	opts := []tablewriter.Option{}

	// Build config
	config := tablewriter.Config{}

	// Apply column alignment if specified
	if len(data.ColumnAlignment) > 0 {
		// Translate table.Align type to tablewriter's tw.Align type
		twAlign := make([]tw.Align, len(data.ColumnAlignment))
		for i, align := range data.ColumnAlignment {
			switch align {
			case table.AlignLeft:
				twAlign[i] = tw.AlignLeft
			case table.AlignCenter:
				twAlign[i] = tw.AlignCenter
			case table.AlignRight:
				twAlign[i] = tw.AlignRight
			default: // table.AlignDefault
				twAlign[i] = tw.Skip
			}
		}

		config.Header.Alignment = tw.CellAlignment{PerColumn: twAlign}
		config.Row.Alignment = tw.CellAlignment{PerColumn: twAlign}
	}

	opts = append(opts, tablewriter.WithConfig(config))
	table := tablewriter.NewTable(w, opts...)

	// Set headers if present
	if len(data.Headers) > 0 {
		// Convert headers to []any for the new API
		headers := make([]any, len(data.Headers))
		for i, h := range data.Headers {
			headers[i] = h
		}
		table.Header(headers...)
	}

	// Add rows
	for _, row := range data.Rows {
		// Convert row to []any for the new API
		rowData := make([]any, len(row))
		for i, cell := range row {
			rowData[i] = cell
		}
		if err := table.Append(rowData...); err != nil {
			return err
		}
	}

	return table.Render()
}

// Data represents data formatted for table output.
type Data struct {
	Headers         []string
	Rows            [][]string
	ColumnAlignment []table.Align // Optional: column alignment (use table.AlignDefault, table.AlignLeft, table.AlignCenter, table.AlignRight)
}

// DetectFormat auto-detects format based on terminal and environment.
func DetectFormat(explicitFormat string) Format {
	// Use explicit format if provided
	if explicitFormat != "" {
		return Format(strings.ToLower(explicitFormat))
	}

	// Check if output is a terminal
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return FormatTable
	}

	// Default to JSON for pipes/redirects
	return FormatJSON
}

// ParseFormat converts string to Format with validation.
func ParseFormat(s string) (Format, error) {
	format := Format(strings.ToLower(s))
	switch format {
	case FormatTable, FormatJSON, FormatYAML, FormatWide, "":
		return format, nil
	default:
		return "", fmt.Errorf("invalid format %q: must be one of: table, json, yaml, wide", s)
	}
}

// convertToTableData attempts to convert struct slices to Data using reflection.
func (f *TableFormatter) convertToTableData(data any) *Data {
	v := reflect.ValueOf(data)

	// Handle slices
	if v.Kind() == reflect.Slice && v.Len() > 0 {
		// Get the first element to determine structure
		firstElem := v.Index(0)
		if firstElem.Kind() == reflect.Struct {
			return f.structSliceToTableData(v)
		}
	}

	// Handle single structs
	if v.Kind() == reflect.Struct {
		return f.singleStructToTableData(v)
	}

	return nil
}

// structSliceToTableData converts a slice of structs to Data.
func (f *TableFormatter) structSliceToTableData(v reflect.Value) *Data {
	if v.Len() == 0 {
		return nil
	}

	firstElem := v.Index(0)
	elemType := firstElem.Type()

	// Extract field names as headers
	var headers []string
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		// Use json tag if available, otherwise field name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove options like ,omitempty
			if idx := strings.Index(jsonTag, ","); idx > 0 {
				jsonTag = jsonTag[:idx]
			}
			caser := cases.Title(language.English)
			headers = append(headers, caser.String(strings.ReplaceAll(jsonTag, "_", " ")))
		} else {
			headers = append(headers, field.Name)
		}
	}

	// Extract data rows
	var rows [][]string
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		var row []string
		for j := 0; j < elem.NumField(); j++ {
			fieldValue := elem.Field(j)
			row = append(row, fmt.Sprintf("%v", fieldValue.Interface()))
		}
		rows = append(rows, row)
	}

	return &Data{
		Headers: headers,
		Rows:    rows,
	}
}

// singleStructToTableData converts a single struct to a key-value table.
func (f *TableFormatter) singleStructToTableData(v reflect.Value) *Data {
	elemType := v.Type()

	headers := []string{"Property", "Value"}
	var rows [][]string

	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		fieldValue := v.Field(i)

		// Use json tag if available, otherwise field name
		var propertyName string
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove options like ,omitempty
			if idx := strings.Index(jsonTag, ","); idx > 0 {
				jsonTag = jsonTag[:idx]
			}
			caser := cases.Title(language.English)
			propertyName = caser.String(strings.ReplaceAll(jsonTag, "_", " "))
		} else {
			propertyName = field.Name
		}

		rows = append(rows, []string{
			propertyName,
			fmt.Sprintf("%v", fieldValue.Interface()),
		})
	}

	return &Data{
		Headers: headers,
		Rows:    rows,
	}
}
