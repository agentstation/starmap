package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"gopkg.in/yaml.v3"
)

// Format types for output
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatWide  Format = "wide"
)

// Formatter interface for all output types
type Formatter interface {
	Format(w io.Writer, data any) error
}

// FormatterFunc allows functions to implement Formatter
type FormatterFunc func(io.Writer, any) error

func (f FormatterFunc) Format(w io.Writer, data any) error {
	return f(w, data)
}

// NewFormatter creates appropriate formatter based on format
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

// JSONFormatter outputs JSON format
type JSONFormatter struct {
	Indent string
}

func (f *JSONFormatter) Format(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	if f.Indent != "" {
		encoder.SetIndent("", f.Indent)
	}
	return encoder.Encode(data)
}

// YAMLFormatter outputs YAML format
type YAMLFormatter struct{}

func (f *YAMLFormatter) Format(w io.Writer, data any) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// TableFormatter outputs table format
type TableFormatter struct {
	Wide bool
}

func (f *TableFormatter) Format(w io.Writer, data any) error {
	// Type switch to handle different data types
	switch v := data.(type) {
	case TableData:
		return f.formatTable(w, v)
	default:
		// Fall back to JSON for non-table data
		jsonFormatter := &JSONFormatter{Indent: "  "}
		return jsonFormatter.Format(w, data)
	}
}

func (f *TableFormatter) formatTable(w io.Writer, data TableData) error {
	// For now, use simple text formatting
	// TODO: Add proper table formatting with tablewriter

	// Print headers
	if len(data.Headers) > 0 {
		for i, header := range data.Headers {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			fmt.Fprintf(w, "%-20s", header)
		}
		fmt.Fprintln(w)

		// Print separator
		for i := range data.Headers {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			fmt.Fprintf(w, "%-20s", strings.Repeat("-", 18))
		}
		fmt.Fprintln(w)
	}

	// Print rows
	for _, row := range data.Rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			// Truncate long cells
			if len(cell) > 18 {
				cell = cell[:15] + "..."
			}
			fmt.Fprintf(w, "%-20s", cell)
		}
		fmt.Fprintln(w)
	}

	return nil
}

// TableData represents data formatted for table output
type TableData struct {
	Headers []string
	Rows    [][]string
}

// DetectFormat auto-detects format based on terminal and environment
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

// ParseFormat converts string to Format with validation
func ParseFormat(s string) (Format, error) {
	format := Format(strings.ToLower(s))
	switch format {
	case FormatTable, FormatJSON, FormatYAML, FormatWide, "":
		return format, nil
	default:
		return "", fmt.Errorf("invalid format %q: must be one of: table, json, yaml, wide", s)
	}
}
