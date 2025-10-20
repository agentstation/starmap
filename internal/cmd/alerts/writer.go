// Package alerts provides structured output writers for different formats.
package alerts

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/cmd/format"
)

// FormatWriter writes alerts in different output formats.
type FormatWriter struct {
	writer io.Writer
	format format.Format
	config WriterConfig
}

// WriterConfig configures alert output behavior.
type WriterConfig struct {
	ShowTimestamp bool
	ShowDetails   bool
	UseColor      bool
}

// NewFormatWriter creates a new FormatWriter for the specified format.
func NewFormatWriter(w io.Writer, fmt format.Format) *FormatWriter {
	return &FormatWriter{
		writer: w,
		format: fmt,
		config: WriterConfig{
			ShowTimestamp: false,
			ShowDetails:   true,
			UseColor:      isTerminal(w),
		},
	}
}

// WithConfig sets the writer configuration.
func (fw *FormatWriter) WithConfig(config WriterConfig) *FormatWriter {
	fw.config = config
	return fw
}

// WriteAlert writes an alert in the configured format.
func (fw *FormatWriter) WriteAlert(alert *Alert) error {
	switch fw.format {
	case format.FormatJSON:
		return fw.writeJSON(alert)
	case format.FormatYAML:
		return fw.writeYAML(alert)
	case format.FormatTable, format.FormatWide:
		return fw.writeTable(alert)
	default:
		return fw.writePlain(alert)
	}
}

// alertData represents alert data for structured output.
type alertData struct {
	Level     string   `json:"level" yaml:"level"`
	Message   string   `json:"message" yaml:"message"`
	Details   []string `json:"details,omitempty" yaml:"details,omitempty"`
	Error     string   `json:"error,omitempty" yaml:"error,omitempty"`
	Timestamp string   `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

func (fw *FormatWriter) toAlertData(alert *Alert) alertData {
	data := alertData{
		Level:   alert.Level.String(),
		Message: alert.Message,
		Details: alert.Details,
	}

	if alert.Err != nil {
		data.Error = alert.Err.Error()
	}

	if fw.config.ShowTimestamp {
		data.Timestamp = alert.Timestamp.Format("2006-01-02T15:04:05Z07:00")
	}

	return data
}

func (fw *FormatWriter) writeJSON(alert *Alert) error {
	data := fw.toAlertData(alert)
	encoder := json.NewEncoder(fw.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (fw *FormatWriter) writeYAML(alert *Alert) error {
	data := fw.toAlertData(alert)
	yamlData, err := yaml.MarshalWithOptions(data,
		yaml.Indent(2),
		yaml.IndentSequence(false),
	)
	if err != nil {
		return err
	}
	_, err = fw.writer.Write(yamlData)
	return err
}

func (fw *FormatWriter) writeTable(alert *Alert) error {
	// Simple, clean output like industry standard CLIs
	icon := alert.Level.Icon()
	message := fmt.Sprintf("%s %s", icon, alert.Message)

	if alert.Err != nil {
		message += fmt.Sprintf(": %v", alert.Err)
	}

	// Just print the message with proper spacing
	_, _ = fmt.Fprintln(fw.writer, message)

	// Add details if present with indentation
	if fw.config.ShowDetails && len(alert.Details) > 0 {
		for _, detail := range alert.Details {
			_, _ = fmt.Fprintf(fw.writer, "   %s\n", detail)
		}
	}

	return nil
}

func (fw *FormatWriter) writePlain(alert *Alert) error {
	// Plain text output with optional color
	message := alert.String()

	if fw.config.UseColor {
		color := alert.Level.Color()
		reset := ResetColor()
		message = fmt.Sprintf("%s%s%s", color, message, reset)
	}

	_, _ = fmt.Fprintln(fw.writer, message)

	// Add details if configured
	if fw.config.ShowDetails && len(alert.Details) > 0 {
		for _, detail := range alert.Details {
			_, _ = fmt.Fprintf(fw.writer, "   %s\n", detail)
		}
	}

	return nil
}

// isTerminal checks if the writer is a terminal (for color support).
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) != 0
	}
	return false
}
