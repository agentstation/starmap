// Package hints provides formatting for hints in different output formats.
package hints

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/goccy/go-yaml"
)

// Formatter formats hints for different output types.
type Formatter struct {
	writer io.Writer
	format output.Format
	config FormatterConfig
}

// FormatterConfig configures hint formatting behavior.
type FormatterConfig struct {
	ShowIcons  bool // Whether to show emoji icons
	MaxWidth   int  // Maximum width for text wrapping
	IndentSize int  // Indentation size for structured output
}

// NewFormatter creates a new hint formatter.
func NewFormatter(w io.Writer, format output.Format) *Formatter {
	return &Formatter{
		writer: w,
		format: format,
		config: FormatterConfig{
			ShowIcons:  true,
			MaxWidth:   80,
			IndentSize: 2,
		},
	}
}

// WithConfig sets the formatter configuration.
func (f *Formatter) WithConfig(config FormatterConfig) *Formatter {
	f.config = config
	return f
}

// FormatHints formats and writes a slice of hints.
func (f *Formatter) FormatHints(hints []*Hint) error {
	if len(hints) == 0 {
		return nil
	}

	switch f.format {
	case output.FormatJSON:
		return f.formatJSON(hints)
	case output.FormatYAML:
		return f.formatYAML(hints)
	case output.FormatTable, output.FormatWide:
		return f.formatTable(hints)
	default:
		return f.formatPlain(hints)
	}
}

// FormatHint formats and writes a single hint.
func (f *Formatter) FormatHint(hint *Hint) error {
	return f.FormatHints([]*Hint{hint})
}

// hintData represents hint data for structured output.
type hintData struct {
	Message string   `json:"message" yaml:"message"`
	Command string   `json:"command,omitempty" yaml:"command,omitempty"`
	URL     string   `json:"url,omitempty" yaml:"url,omitempty"`
	Tags    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

func (f *Formatter) toHintData(hint *Hint) hintData {
	return hintData{
		Message: hint.Message,
		Command: hint.Command,
		URL:     hint.URL,
		Tags:    hint.Tags,
	}
}

func (f *Formatter) formatJSON(hints []*Hint) error {
	data := make([]hintData, len(hints))
	for i, hint := range hints {
		data[i] = f.toHintData(hint)
	}

	// Wrap in hints object for clarity
	output := struct {
		Hints []hintData `json:"hints"`
	}{
		Hints: data,
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", strings.Repeat(" ", f.config.IndentSize))
	return encoder.Encode(output)
}

func (f *Formatter) formatYAML(hints []*Hint) error {
	data := make([]hintData, len(hints))
	for i, hint := range hints {
		data[i] = f.toHintData(hint)
	}

	// Wrap in hints object for clarity
	output := struct {
		Hints []hintData `yaml:"hints"`
	}{
		Hints: data,
	}

	yamlData, err := yaml.MarshalWithOptions(output,
		yaml.Indent(f.config.IndentSize),
		yaml.IndentSequence(false),
	)
	if err != nil {
		return err
	}
	_, err = f.writer.Write(yamlData)
	return err
}

func (f *Formatter) formatTable(hints []*Hint) error {
	// Simple, clean hints like GitHub CLI
	if len(hints) == 0 {
		return nil
	}

	_, _ = fmt.Fprintln(f.writer) // One newline before

	for _, hint := range hints {
		lines := f.formatHintContent(hint)
		for _, line := range lines {
			_, _ = fmt.Fprintln(f.writer, line)
		}
	}

	_, _ = fmt.Fprintln(f.writer) // One newline after

	return nil
}

func (f *Formatter) formatPlain(hints []*Hint) error {
	if len(hints) == 0 {
		return nil
	}

	_, _ = fmt.Fprintln(f.writer) // One newline before

	for _, hint := range hints {
		lines := f.formatHintContent(hint)
		for _, line := range lines {
			_, _ = fmt.Fprintln(f.writer, line)
		}
	}

	_, _ = fmt.Fprintln(f.writer) // One newline after

	return nil
}

// formatHintContent formats the content of a single hint into lines.
func (f *Formatter) formatHintContent(hint *Hint) []string {
	var lines []string

	// Main message
	icon := "💡"
	if !f.config.ShowIcons {
		icon = "Tip:"
	}

	message := fmt.Sprintf("%s %s", icon, hint.Message)
	lines = append(lines, message)

	// Command if present
	if hint.Command != "" {
		lines = append(lines, fmt.Sprintf("   Run: %s", hint.Command))
	}

	// URL if present
	if hint.URL != "" {
		lines = append(lines, fmt.Sprintf("   See: %s", hint.URL))
	}

	return lines
}

// Display is a convenience function to format and display hints.
func Display(w io.Writer, format output.Format, hints []*Hint) error {
	formatter := NewFormatter(w, format)
	return formatter.FormatHints(hints)
}

// DisplayHint is a convenience function to format and display a single hint.
func DisplayHint(w io.Writer, format output.Format, hint *Hint) error {
	return Display(w, format, []*Hint{hint})
}
