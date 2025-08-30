package logging

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// TestLogger creates a test logger that captures output
type TestLogger struct {
	*zerolog.Logger
	Buffer *bytes.Buffer
}

// NewTestLogger creates a new test logger that captures output
func NewTestLogger(t testing.TB) *TestLogger {
	t.Helper()
	
	buf := &bytes.Buffer{}
	// Set global level to trace to capture everything
	oldLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	
	logger := zerolog.New(buf).
		Level(zerolog.TraceLevel). // Capture all levels in tests
		With().
		Timestamp().
		Logger()
	
	// Restore level on cleanup
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(oldLevel)
	})
	
	return &TestLogger{
		Logger: &logger,
		Buffer: buf,
	}
}

// Output returns the captured log output as a string
func (tl *TestLogger) Output() string {
	return tl.Buffer.String()
}

// Lines returns the captured log output as individual lines
func (tl *TestLogger) Lines() []string {
	output := strings.TrimSpace(tl.Output())
	if output == "" {
		return []string{}
	}
	return strings.Split(output, "\n")
}

// Contains checks if the log output contains the given string
func (tl *TestLogger) Contains(substr string) bool {
	return strings.Contains(tl.Output(), substr)
}

// ContainsAll checks if the log output contains all given strings
func (tl *TestLogger) ContainsAll(substrs ...string) bool {
	output := tl.Output()
	for _, substr := range substrs {
		if !strings.Contains(output, substr) {
			return false
		}
	}
	return true
}

// ContainsAny checks if the log output contains any of the given strings
func (tl *TestLogger) ContainsAny(substrs ...string) bool {
	output := tl.Output()
	for _, substr := range substrs {
		if strings.Contains(output, substr) {
			return true
		}
	}
	return false
}

// Count returns the number of log entries
func (tl *TestLogger) Count() int {
	return len(tl.Lines())
}

// Clear clears the captured log output
func (tl *TestLogger) Clear() {
	tl.Buffer.Reset()
}

// AssertContains asserts that the log contains the given string
func (tl *TestLogger) AssertContains(t testing.TB, substr string) {
	t.Helper()
	if !tl.Contains(substr) {
		t.Errorf("Log output does not contain %q\nOutput:\n%s", substr, tl.Output())
	}
}

// AssertNotContains asserts that the log does not contain the given string
func (tl *TestLogger) AssertNotContains(t testing.TB, substr string) {
	t.Helper()
	if tl.Contains(substr) {
		t.Errorf("Log output should not contain %q\nOutput:\n%s", substr, tl.Output())
	}
}

// AssertCount asserts that the log has the expected number of entries
func (tl *TestLogger) AssertCount(t testing.TB, expected int) {
	t.Helper()
	actual := tl.Count()
	if actual != expected {
		t.Errorf("Expected %d log entries, got %d\nOutput:\n%s", expected, actual, tl.Output())
	}
}

// NewNopLogger creates a logger that discards all output (useful for tests)
func NewNopLogger() *zerolog.Logger {
	logger := zerolog.Nop()
	return &logger
}

// DisableLoggingForTest disables logging for the duration of a test
func DisableLoggingForTest(t testing.TB) {
	t.Helper()
	
	// Save current logger
	original := Default()
	
	// Set to nop logger
	SetDefault(zerolog.Nop())
	
	// Restore on cleanup
	t.Cleanup(func() {
		SetDefault(*original)
	})
}

// CaptureLoggingForTest captures logging output for the duration of a test
func CaptureLoggingForTest(t testing.TB) *TestLogger {
	t.Helper()
	
	// Save current logger
	original := Default()
	
	// Create test logger
	testLogger := NewTestLogger(t)
	
	// Set as default
	SetDefault(*testLogger.Logger)
	
	// Restore on cleanup
	t.Cleanup(func() {
		SetDefault(*original)
	})
	
	return testLogger
}