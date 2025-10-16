// Package alerts provides a structured system for status notifications.
package alerts

import (
	"fmt"
	"github.com/agentstation/starmap/internal/cmd/emoji"
)

// Level represents the severity of an alert.
type Level int

const (
	// LevelError indicates a failure or error condition.
	LevelError Level = iota
	// LevelWarning indicates a potential issue or important notice.
	LevelWarning
	// LevelInfo indicates general informational messages.
	LevelInfo
	// LevelSuccess indicates successful completion of an operation.
	LevelSuccess
)

// String returns the string representation of the alert level.
func (l Level) String() string {
	switch l {
	case LevelError:
		return "error"
	case LevelWarning:
		return "warning"
	case LevelInfo:
		return "info"
	case LevelSuccess:
		return "success"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// Icon returns the appropriate icon for the alert level.
func (l Level) Icon() string {
	switch l {
	case LevelError:
		return emoji.Error + ""
	case LevelWarning:
		return "⚠️"
	case LevelInfo:
		return "ℹ️"
	case LevelSuccess:
		return emoji.Success + ""
	default:
		return "❓"
	}
}

// Color returns ANSI color codes for terminal output.
func (l Level) Color() string {
	switch l {
	case LevelError:
		return "\033[31m" // Red
	case LevelWarning:
		return "\033[33m" // Yellow
	case LevelInfo:
		return "\033[36m" // Cyan
	case LevelSuccess:
		return "\033[32m" // Green
	default:
		return "\033[0m" // Reset
	}
}

// ResetColor returns the ANSI reset code.
func ResetColor() string {
	return "\033[0m"
}
