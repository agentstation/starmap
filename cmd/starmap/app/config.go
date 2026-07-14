package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const envDisableDotenv = "STARMAP_DISABLE_DOTENV"

// Config holds the application configuration loaded from various sources
// including config files, environment variables, and .env files.
type Config struct {
	// Global flags
	Verbose bool
	Quiet   bool
	NoColor bool
	Output  string

	// Config file
	ConfigFile string

	// Starmap configuration
	// CatalogExportPath is an optional editable YAML import/export tree.
	CatalogExportPath string
	// CatalogPath is the durable canonical catalog database root.
	CatalogPath                   string
	UseEmbeddedCatalog            bool
	EmbeddedBootstrapMaxAge       time.Duration
	EmbeddedBootstrapMaxSizeBytes int64
	RemoteServerURL               string
	RemoteServerAPIKey            string
	RemoteServerOnly              bool

	// Logging configuration
	LogLevel  string
	LogFormat string
	LogOutput string
}

// LoadConfig loads configuration from all sources in order of precedence:
// 1. Command-line flags (handled by cobra)
// 2. Environment variables
// 3. .env files
// 4. Config file (~/.starmap/config.yaml)
// 5. Defaults.
func LoadConfig() (*Config, error) {
	// Load .env files first (before Viper env binding)
	loadEnvFiles()
	viper.Reset()

	// Set up Viper for environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	_ = viper.BindEnv("output", "OUTPUT")

	// Try to read config file if it exists
	configFile := viper.GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Use the canonical namespaced configuration file.
		home, err := os.UserHomeDir()
		if err == nil {
			viper.SetConfigFile(filepath.Join(home, ".starmap", "config.yaml"))
		}
	}

	// Read config file (ignore error if not found).
	configFileUsed := ""
	if err := viper.ReadInConfig(); err == nil {
		configFileUsed = viper.ConfigFileUsed()
	}

	// Build config from viper
	config := &Config{
		// Global flags (may be overridden by cobra flags later)
		Verbose: viper.GetBool("verbose"),
		Quiet:   viper.GetBool("quiet"),
		NoColor: viper.GetBool("no-color"),
		Output:  viper.GetString("output"),

		// Config file
		ConfigFile: configFileUsed,

		// Starmap configuration
		CatalogExportPath:             viper.GetString("catalog_export_path"),
		CatalogPath:                   viper.GetString("catalog_path"),
		UseEmbeddedCatalog:            viper.GetBool("use_embedded_catalog"),
		EmbeddedBootstrapMaxAge:       viper.GetDuration("embedded_bootstrap_max_age"),
		EmbeddedBootstrapMaxSizeBytes: viper.GetInt64("embedded_bootstrap_max_size_bytes"),
		RemoteServerURL:               viper.GetString("remote_server_url"),
		RemoteServerAPIKey:            viper.GetString("remote_server_api_key"),
		RemoteServerOnly:              viper.GetBool("remote_server_only"),

		// Logging configuration
		// LogLevel: empty string means "use precedence logic" (see logger.go)
		// If LOG_LEVEL env var is set, it will be used; otherwise defaults to "info" via precedence
		LogLevel:  getEnvOrDefault("LOG_LEVEL", ""),
		LogFormat: getEnvOrDefault("LOG_FORMAT", "auto"),
		LogOutput: getEnvOrDefault("LOG_OUTPUT", "stderr"),
	}

	return config, nil
}

// UpdateFromFlags updates config values from parsed command flags.
// This should be called after cobra parses flags to ensure flag
// values take precedence over config file and env vars.
func (c *Config) UpdateFromFlags(verbose, quiet, noColor bool, output, logLevel string) {
	c.Verbose = verbose
	c.Quiet = quiet
	c.NoColor = noColor
	if output != "" {
		c.Output = output
	}
	if logLevel != "" {
		c.LogLevel = logLevel
	}
}

// loadEnvFiles loads environment variables from .env files.
func loadEnvFiles() {
	if os.Getenv(envDisableDotenv) != "" {
		return
	}
	// Load the higher-precedence local file first. godotenv.Load never replaces
	// an already-present process value, so process environment remains highest.
	envFiles := []string{
		".env.local",
		".env",
	}

	for _, envFile := range envFiles {
		_ = godotenv.Load(envFile)
	}
}

// getEnvOrDefault returns the environment variable value or the default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
