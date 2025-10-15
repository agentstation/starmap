package app

import (
	"testing"
)

// TestDetermineLogLevel tests the log level precedence logic.
func TestDetermineLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "default level when no flags set",
			config: &Config{
				LogLevel: "",
				Verbose:  false,
				Quiet:    false,
			},
			expected: "info",
		},
		{
			name: "verbose flag sets debug",
			config: &Config{
				LogLevel: "",
				Verbose:  true,
				Quiet:    false,
			},
			expected: "debug",
		},
		{
			name: "quiet flag sets warn",
			config: &Config{
				LogLevel: "",
				Verbose:  false,
				Quiet:    true,
			},
			expected: "warn",
		},
		{
			name: "explicit log-level overrides verbose",
			config: &Config{
				LogLevel: "error",
				Verbose:  true,
				Quiet:    false,
			},
			expected: "error",
		},
		{
			name: "explicit log-level overrides quiet",
			config: &Config{
				LogLevel: "trace",
				Verbose:  false,
				Quiet:    true,
			},
			expected: "trace",
		},
		{
			name: "explicit log-level overrides both flags",
			config: &Config{
				LogLevel: "info",
				Verbose:  true,
				Quiet:    true,
			},
			expected: "info",
		},
		{
			name: "both verbose and quiet prefers quiet",
			config: &Config{
				LogLevel: "",
				Verbose:  true,
				Quiet:    true,
			},
			expected: "warn",
		},
		{
			name: "env var LOG_LEVEL read from config",
			config: &Config{
				LogLevel: "debug", // Simulates LOG_LEVEL=debug env var
				Verbose:  false,
				Quiet:    false,
			},
			expected: "debug",
		},
		{
			name: "invalid log level falls back to info",
			config: &Config{
				LogLevel: "invalid",
				Verbose:  false,
				Quiet:    false,
			},
			expected: "info",
		},
		{
			name: "trace level supported",
			config: &Config{
				LogLevel: "trace",
				Verbose:  false,
				Quiet:    false,
			},
			expected: "trace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineLogLevel(tt.config)
			if result != tt.expected {
				t.Errorf("determineLogLevel() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestValidateLogLevel tests log level validation.
func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "valid trace",
			level:    "trace",
			expected: "trace",
		},
		{
			name:     "valid debug",
			level:    "debug",
			expected: "debug",
		},
		{
			name:     "valid info",
			level:    "info",
			expected: "info",
		},
		{
			name:     "valid warn",
			level:    "warn",
			expected: "warn",
		},
		{
			name:     "valid error",
			level:    "error",
			expected: "error",
		},
		{
			name:     "invalid level returns info",
			level:    "invalid",
			expected: "info",
		},
		{
			name:     "empty string returns info",
			level:    "",
			expected: "info",
		},
		{
			name:     "uppercase returns info",
			level:    "DEBUG",
			expected: "info",
		},
		{
			name:     "mixed case returns info",
			level:    "Debug",
			expected: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateLogLevel(tt.level)
			if result != tt.expected {
				t.Errorf("validateLogLevel(%q) = %q, expected %q", tt.level, result, tt.expected)
			}
		})
	}
}

// TestNewLogger tests that logger creation works with various configs.
func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "default config",
			config: &Config{
				LogLevel:  "",
				LogFormat: "auto",
				LogOutput: "stderr",
				Verbose:   false,
				Quiet:     false,
			},
		},
		{
			name: "verbose mode",
			config: &Config{
				LogLevel:  "",
				LogFormat: "auto",
				LogOutput: "stderr",
				Verbose:   true,
				Quiet:     false,
			},
		},
		{
			name: "quiet mode",
			config: &Config{
				LogLevel:  "",
				LogFormat: "auto",
				LogOutput: "stderr",
				Verbose:   false,
				Quiet:     true,
			},
		},
		{
			name: "explicit trace level",
			config: &Config{
				LogLevel:  "trace",
				LogFormat: "auto",
				LogOutput: "stderr",
				Verbose:   false,
				Quiet:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic - just verify logger creation succeeds
			_ = NewLogger(tt.config)
		})
	}
}

// TestLogLevelPrecedenceOrder tests the documented precedence order.
func TestLogLevelPrecedenceOrder(t *testing.T) {
	// Test 1: --log-level beats everything
	config1 := &Config{
		LogLevel: "error",
		Verbose:  true,
		Quiet:    true,
	}
	if level := determineLogLevel(config1); level != "error" {
		t.Errorf("--log-level should win over flags, got %q", level)
	}

	// Test 2: -v beats -q when both set
	config2 := &Config{
		LogLevel: "",
		Verbose:  true,
		Quiet:    true,
	}
	if level := determineLogLevel(config2); level != "warn" {
		t.Errorf("conflicting flags should use -q, got %q", level)
	}

	// Test 3: -v works when no explicit level
	config3 := &Config{
		LogLevel: "",
		Verbose:  true,
		Quiet:    false,
	}
	if level := determineLogLevel(config3); level != "debug" {
		t.Errorf("-v should set debug, got %q", level)
	}

	// Test 4: -q works when no explicit level
	config4 := &Config{
		LogLevel: "",
		Verbose:  false,
		Quiet:    true,
	}
	if level := determineLogLevel(config4); level != "warn" {
		t.Errorf("-q should set warn, got %q", level)
	}

	// Test 5: default when nothing set
	config5 := &Config{
		LogLevel: "",
		Verbose:  false,
		Quiet:    false,
	}
	if level := determineLogLevel(config5); level != "info" {
		t.Errorf("default should be info, got %q", level)
	}
}
