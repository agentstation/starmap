package save

import "io"

type Format int

// Format constants.
const (
	FormatJSON Format = iota
	FormatYAML
)

// IsValid checks if the format is valid.
func (f Format) IsValid() bool {
	switch f {
	case FormatJSON, FormatYAML:
		return true
	default:
		return false
	}
}

// String returns the string representation of the format.
func (f Format) String() string {
	switch f {
	case FormatJSON:
		return "json"
	case FormatYAML:
		return "yaml"
	}
	return "unknown"
}

// Options is the configuration for save.
type Options struct {
	path   string
	writer io.Writer
	format Format
}

// Path returns the path for the save options.
func (s *Options) Path() string {
	return s.path
}

// Writer returns the writer for the save options.
func (s *Options) Writer() io.Writer {
	return s.writer
}

// Format returns the format for the save options.
func (s *Options) Format() Format {
	return s.format
}

// Defaults returns the default save options.
func Defaults() *Options {
	return &Options{
		path:   "",
		writer: nil,
		format: FormatJSON,
	}
}

// Apply applies the given options to the save options.
func (s *Options) Apply(opts ...Option) Options {
	for _, opt := range opts {
		opt(s)
	}
	return *s
}

// Option is a function that configures save options.
type Option func(*Options)

// WithFormat for custom output format.
func WithFormat(f Format) Option {
	return func(s *Options) {
		s.format = f
	}
}

// WithPath for filesystem saves.
func WithPath(path string) Option {
	return func(s *Options) {
		s.path = path
	}
}

// WithWriter for custom outputs.
func WithWriter(w io.Writer) Option {
	return func(s *Options) {
		s.writer = w
	}
}
