package app

import (
	"os"
	"testing"
	"time"
)

// TestLoadConfig verifies basic config loading.
func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Verify defaults are set
	// Note: LogLevel may be empty (triggers precedence logic in logger.go)
	// LogFormat should have a default
	if config.LogFormat == "" {
		t.Error("LogFormat not set to default")
	}
}

// TestConfig_EnvironmentVariables verifies environment variable loading.
func TestConfig_EnvironmentVariables(t *testing.T) {
	// Save original env
	oldVerbose := os.Getenv("VERBOSE")
	oldOutput := os.Getenv("OUTPUT")
	defer func() {
		os.Setenv("VERBOSE", oldVerbose)
		os.Setenv("OUTPUT", oldOutput)
	}()

	// Set test environment variables
	os.Setenv("VERBOSE", "true")
	os.Setenv("OUTPUT", "json")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if !config.Verbose {
		t.Error("VERBOSE environment variable not loaded")
	}
	if config.Output != "json" {
		t.Errorf("OUTPUT = %s, want json", config.Output)
	}
}

// TestConfig_AutoUpdateInterval verifies time duration parsing.
func TestConfig_AutoUpdateInterval(t *testing.T) {
	// Save original env
	oldInterval := os.Getenv("AUTO_UPDATE_INTERVAL")
	defer os.Setenv("AUTO_UPDATE_INTERVAL", oldInterval)

	// Set test interval
	os.Setenv("AUTO_UPDATE_INTERVAL", "1h")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config.AutoUpdateInterval != time.Hour {
		t.Errorf("AutoUpdateInterval = %v, want 1h", config.AutoUpdateInterval)
	}
}

// TestConfig_BooleanFlags verifies boolean flag parsing.
func TestConfig_BooleanFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		check    func(*Config) bool
		want     bool
	}{
		{
			name:     "UseEmbeddedCatalog",
			envVar:   "USE_EMBEDDED_CATALOG",
			envValue: "true",
			check:    func(c *Config) bool { return c.UseEmbeddedCatalog },
			want:     true,
		},
		{
			name:     "AutoUpdatesEnabled",
			envVar:   "AUTO_UPDATES_ENABLED",
			envValue: "false",
			check:    func(c *Config) bool { return c.AutoUpdatesEnabled },
			want:     false,
		},
		{
			name:     "NoColor",
			envVar:   "NO_COLOR",
			envValue: "1",
			check:    func(c *Config) bool { return c.NoColor },
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			old := os.Getenv(tt.envVar)
			defer os.Setenv(tt.envVar, old)

			os.Setenv(tt.envVar, tt.envValue)

			config, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			got := tt.check(config)
			if got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestConfig_RemoteServer verifies remote server configuration.
func TestConfig_RemoteServer(t *testing.T) {
	// Save original env
	oldURL := os.Getenv("REMOTE_SERVER_URL")
	oldKey := os.Getenv("REMOTE_SERVER_API_KEY")
	oldOnly := os.Getenv("REMOTE_SERVER_ONLY")
	defer func() {
		os.Setenv("REMOTE_SERVER_URL", oldURL)
		os.Setenv("REMOTE_SERVER_API_KEY", oldKey)
		os.Setenv("REMOTE_SERVER_ONLY", oldOnly)
	}()

	// Set test values
	os.Setenv("REMOTE_SERVER_URL", "https://api.example.com")
	os.Setenv("REMOTE_SERVER_API_KEY", "test-key-123")
	os.Setenv("REMOTE_SERVER_ONLY", "true")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config.RemoteServerURL != "https://api.example.com" {
		t.Errorf("RemoteServerURL = %s, want https://api.example.com", config.RemoteServerURL)
	}
	if config.RemoteServerAPIKey != "test-key-123" {
		t.Errorf("RemoteServerAPIKey = %s, want test-key-123", config.RemoteServerAPIKey)
	}
	if !config.RemoteServerOnly {
		t.Error("RemoteServerOnly = false, want true")
	}
}

// TestConfig_LoggingOptions verifies logging configuration.
func TestConfig_LoggingOptions(t *testing.T) {
	// Save original env
	oldLevel := os.Getenv("LOG_LEVEL")
	oldFormat := os.Getenv("LOG_FORMAT")
	oldOutput := os.Getenv("LOG_OUTPUT")
	defer func() {
		os.Setenv("LOG_LEVEL", oldLevel)
		os.Setenv("LOG_FORMAT", oldFormat)
		os.Setenv("LOG_OUTPUT", oldOutput)
	}()

	// Set test values
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("LOG_OUTPUT", "stdout")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config.LogLevel != "debug" {
		t.Errorf("LogLevel = %s, want debug", config.LogLevel)
	}
	if config.LogFormat != "json" {
		t.Errorf("LogFormat = %s, want json", config.LogFormat)
	}
	if config.LogOutput != "stdout" {
		t.Errorf("LogOutput = %s, want stdout", config.LogOutput)
	}
}

// TestConfig_LocalPath verifies local path configuration.
func TestConfig_LocalPath(t *testing.T) {
	// Save original env
	oldPath := os.Getenv("LOCAL_PATH")
	defer os.Setenv("LOCAL_PATH", oldPath)

	// Set test value
	testPath := "/tmp/starmap-test"
	os.Setenv("LOCAL_PATH", testPath)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config.LocalPath != testPath {
		t.Errorf("LocalPath = %s, want %s", config.LocalPath, testPath)
	}
}
