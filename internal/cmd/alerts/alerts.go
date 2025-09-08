// Package alerts provides a structured system for status notifications.
package alerts

import (
	"fmt"
	"io"
	"time"
)

// Alert represents a system status notification.
type Alert struct {
	Level     Level
	Message   string
	Details   []string
	Timestamp time.Time
	Err       error
}

// New creates a new alert with the given level and message.
func New(level Level, message string) *Alert {
	return &Alert{
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// NewError creates a new error alert.
func NewError(message string) *Alert {
	return New(LevelError, message)
}

// NewWarning creates a new warning alert.
func NewWarning(message string) *Alert {
	return New(LevelWarning, message)
}

// NewInfo creates a new info alert.
func NewInfo(message string) *Alert {
	return New(LevelInfo, message)
}

// NewSuccess creates a new success alert.
func NewSuccess(message string) *Alert {
	return New(LevelSuccess, message)
}

// WithError adds an underlying error to the alert.
func (a *Alert) WithError(err error) *Alert {
	a.Err = err
	return a
}

// WithDetails adds additional context details to the alert.
func (a *Alert) WithDetails(details ...string) *Alert {
	a.Details = append(a.Details, details...)
	return a
}

// String returns a string representation of the alert.
func (a *Alert) String() string {
	icon := a.Level.Icon()
	message := fmt.Sprintf("%s %s", icon, a.Message)
	
	if a.Err != nil {
		message += fmt.Sprintf(": %v", a.Err)
	}
	
	return message
}

// Writer handles alert output to different formats and destinations.
type Writer interface {
	WriteAlert(alert *Alert) error
}

// WriterFunc is an adapter to allow functions to be used as Writers.
type WriterFunc func(*Alert) error

// WriteAlert calls the function.
func (f WriterFunc) WriteAlert(alert *Alert) error {
	return f(alert)
}

// MultiWriter creates a writer that writes to multiple writers.
func MultiWriter(writers ...Writer) Writer {
	return WriterFunc(func(alert *Alert) error {
		for _, w := range writers {
			if err := w.WriteAlert(alert); err != nil {
				return err
			}
		}
		return nil
	})
}

// DiscardWriter is a Writer that discards all alerts.
var DiscardWriter Writer = WriterFunc(func(*Alert) error { return nil })

// NewWriterTo creates a Writer that writes to an io.Writer.
func NewWriterTo(w io.Writer) Writer {
	return WriterFunc(func(alert *Alert) error {
		_, err := fmt.Fprintln(w, alert.String())
		return err
	})
}