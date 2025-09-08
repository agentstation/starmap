// Package alerts provides structured output writers for different formats.
package alerts

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/cmd/output"
	"gopkg.in/yaml.v3"
)

// FormatWriter writes alerts in different output formats.
type FormatWriter struct {
	writer io.Writer
	format output.Format
	config WriterConfig
}

// WriterConfig configures alert output behavior.
type WriterConfig struct {
	ShowTimestamp bool
	ShowDetails   bool
	UseColor      bool
}

// NewFormatWriter creates a new FormatWriter for the specified format.
func NewFormatWriter(w io.Writer, format output.Format) *FormatWriter {
	return &FormatWriter{
		writer: w,
		format: format,
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
	case output.FormatJSON:
		return fw.writeJSON(alert)
	case output.FormatYAML:
		return fw.writeYAML(alert)
	case output.FormatTable, output.FormatWide:
		return fw.writeTable(alert)
	default:
		return fw.writePlain(alert)
	}
}

// alertData represents alert data for structured output.
type alertData struct {
	Level     string    `json:"level" yaml:"level"`
	Message   string    `json:"message" yaml:"message"`
	Details   []string  `json:"details,omitempty" yaml:"details,omitempty"`
	Error     string    `json:"error,omitempty" yaml:"error,omitempty"`
	Timestamp string    `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
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
	encoder := yaml.NewEncoder(fw.writer)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(data)
}

func (fw *FormatWriter) writeTable(alert *Alert) error {
	// Simple, clean output like industry standard CLIs
	icon := alert.Level.Icon()
	message := fmt.Sprintf("%s %s", icon, alert.Message)
	
	if alert.Err != nil {
		message += fmt.Sprintf(": %v", alert.Err)
	}
	
	// Just print the message with proper spacing
	fmt.Fprintln(fw.writer, message)
	
	// Add details if present with indentation
	if fw.config.ShowDetails && len(alert.Details) > 0 {
		for _, detail := range alert.Details {
			fmt.Fprintf(fw.writer, "   %s\n", detail)
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
	
	fmt.Fprintln(fw.writer, message)
	
	// Add details if configured
	if fw.config.ShowDetails && len(alert.Details) > 0 {
		for _, detail := range alert.Details {
			fmt.Fprintf(fw.writer, "   %s\n", detail)
		}
	}
	
	return nil
}

// repeatString repeats a string n times.
func repeatString(s string, count int) string {
	if count <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
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