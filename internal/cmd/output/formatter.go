// Package output provides formatters for command output.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
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
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// TableFormatter outputs table format.
type TableFormatter struct {
	Wide bool
}

// Format outputs data in table format.
func (f *TableFormatter) Format(w io.Writer, data any) error {
	// Type switch to handle different data types
	switch v := data.(type) {
	case TableData:
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

func (f *TableFormatter) formatTable(w io.Writer, data TableData) error {
	// Use tablewriter for proper table formatting
	table := tablewriter.NewTable(w)

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

// TableData represents data formatted for table output.
// Deprecated: This type name stutters with package name.
// TODO: Refactor to use table.Data from internal/cmd/table package.
type TableData struct {
	Headers []string
	Rows    [][]string
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

// convertToTableData attempts to convert struct slices to TableData using reflection.
func (f *TableFormatter) convertToTableData(data any) *TableData {
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

// structSliceToTableData converts a slice of structs to TableData.
func (f *TableFormatter) structSliceToTableData(v reflect.Value) *TableData {
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

	return &TableData{
		Headers: headers,
		Rows:    rows,
	}
}

// singleStructToTableData converts a single struct to a key-value table.
func (f *TableFormatter) singleStructToTableData(v reflect.Value) *TableData {
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

	return &TableData{
		Headers: headers,
		Rows:    rows,
	}
}
