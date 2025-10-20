// Package notify provides a unified API for alerts and hints in the CLI.
package notify

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/alerts"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/hints"
)

// Notifier is the main public API for sending alerts and displaying hints.
type Notifier struct {
	alertWriter  alerts.Writer
	hintRegistry *hints.Registry
	config       Config
}

// Config controls notification behavior.
type Config struct {
	OutputFormat string    // "table", "json", "yaml"
	ShowHints    bool      // Whether to show hints
	ShowAlerts   bool      // Whether to show alerts
	MaxHints     int       // Maximum number of hints to show
	AlertWriter  io.Writer // Where to write alerts (default: stderr)
	HintWriter   io.Writer // Where to write hints (default: stdout)
	UseColor     bool      // Whether to use colored output
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		OutputFormat: "auto", // Will be detected
		ShowHints:    true,
		ShowAlerts:   true,
		MaxHints:     1, // Show only the most valuable hint
		AlertWriter:  os.Stderr,
		HintWriter:   os.Stdout,
		UseColor:     true,
	}
}

// New creates a new Notifier with the given configuration.
func New(config Config) *Notifier {
	// Set up alert writer
	format := detectOutputFormat(config.OutputFormat)
	alertWriter := alerts.NewFormatWriter(config.AlertWriter, format)

	// Set up hint registry with starmap providers
	hintRegistry := hints.NewRegistry().WithConfig(hints.RegistryConfig{
		MaxHints: config.MaxHints,
		Enabled:  config.ShowHints,
	})
	hints.RegisterStarmapProviders(hintRegistry)

	return &Notifier{
		alertWriter:  alertWriter,
		hintRegistry: hintRegistry,
		config:       config,
	}
}

// NewFromCommand creates a Notifier configured from a Cobra command.
func NewFromCommand(cmd *cobra.Command) (*Notifier, error) {
	config := DefaultConfig()

	// Get global flags
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse global flags: %w", err)
	}

	// Configure from flags
	config.OutputFormat = globalFlags.Format
	config.ShowHints = !globalFlags.Quiet && !isCI()
	config.UseColor = !globalFlags.NoColor && isTerminal(os.Stdout)

	return New(config), nil
}

// Alert sends an alert notification.
func (n *Notifier) Alert(alert *alerts.Alert) error {
	if !n.config.ShowAlerts {
		return nil
	}

	return n.alertWriter.WriteAlert(alert)
}

// Success sends a success alert with optional hints.
func (n *Notifier) Success(message string, ctx hints.Context) error {
	return n.AlertWithHints(alerts.NewSuccess(message), ctx)
}

// Error sends an error alert with optional hints.
func (n *Notifier) Error(message string, ctx hints.Context) error {
	return n.AlertWithHints(alerts.NewError(message), ctx)
}

// Warning sends a warning alert with optional hints.
func (n *Notifier) Warning(message string, ctx hints.Context) error {
	return n.AlertWithHints(alerts.NewWarning(message), ctx)
}

// Info sends an info alert with optional hints.
func (n *Notifier) Info(message string, ctx hints.Context) error {
	return n.AlertWithHints(alerts.NewInfo(message), ctx)
}

// AlertWithHints sends an alert and displays contextual hints.
func (n *Notifier) AlertWithHints(alert *alerts.Alert, ctx hints.Context) error {
	// Send the alert
	if err := n.Alert(alert); err != nil {
		return fmt.Errorf("failed to write alert: %w", err)
	}

	// Display contextual hints
	if n.config.ShowHints {
		hintList := n.hintRegistry.GetHints(ctx)
		if len(hintList) > 0 {
			format := detectOutputFormat(n.config.OutputFormat)
			return hints.Display(n.config.HintWriter, format, hintList)
		}
	}

	return nil
}

// Hints displays contextual hints without an alert.
func (n *Notifier) Hints(ctx hints.Context) error {
	if !n.config.ShowHints {
		return nil
	}

	hintList := n.hintRegistry.GetHints(ctx)
	if len(hintList) == 0 {
		return nil
	}

	format := detectOutputFormat(n.config.OutputFormat)
	return hints.Display(n.config.HintWriter, format, hintList)
}

// AddHintProvider registers a custom hint provider.
func (n *Notifier) AddHintProvider(provider hints.Provider) {
	n.hintRegistry.Register(provider)
}

// AddHintFunc registers a function as a hint provider.
func (n *Notifier) AddHintFunc(name string, fn func(hints.Context) []*hints.Hint) {
	n.hintRegistry.RegisterFunc(name, fn)
}

// WithConfig creates a new Notifier with updated configuration.
func (n *Notifier) WithConfig(config Config) *Notifier {
	return New(config)
}

// detectOutputFormat determines the output format from a string.
func detectOutputFormat(formatStr string) format.Format {
	if formatStr == "" || formatStr == "auto" {
		return format.DetectFormat("")
	}
	return format.Format(strings.ToLower(formatStr))
}

// isTerminal checks if the given writer is a terminal.
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

// isCI detects if running in a CI/CD environment.
func isCI() bool {
	ciEnvVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS_URL",
		"BUILDKITE",
		"TRAVIS",
		"CIRCLECI",
	}

	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	return false
}

// Global notifier instance for convenience functions.
var defaultNotifier *Notifier

// SetDefault sets the default global notifier.
func SetDefault(notifier *Notifier) {
	defaultNotifier = notifier
}

// GetDefault returns the default global notifier, creating one if needed.
func GetDefault() *Notifier {
	if defaultNotifier == nil {
		defaultNotifier = New(DefaultConfig())
	}
	return defaultNotifier
}

// Convenience functions using the default notifier

// Success sends a success alert using the default notifier.
func Success(message string, ctx hints.Context) error {
	return GetDefault().Success(message, ctx)
}

// Error sends an error alert using the default notifier.
func Error(message string, ctx hints.Context) error {
	return GetDefault().Error(message, ctx)
}

// Warning sends a warning alert using the default notifier.
func Warning(message string, ctx hints.Context) error {
	return GetDefault().Warning(message, ctx)
}

// Info sends an info alert using the default notifier.
func Info(message string, ctx hints.Context) error {
	return GetDefault().Info(message, ctx)
}

// Alert sends an alert using the default notifier.
func Alert(alert *alerts.Alert) error {
	return GetDefault().Alert(alert)
}

// Hints displays hints using the default notifier.
func Hints(ctx hints.Context) error {
	return GetDefault().Hints(ctx)
}
