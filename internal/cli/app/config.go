package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

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
	// CatalogPath is the human-editable provider YAML workspace.
	CatalogPath                   string
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
	return loadConfig("")
}

// loadConfig loads configuration, using configFile when it is non-empty.
// Explicit files are required to exist and parse successfully; the default
// file remains optional.
func loadConfig(configFile string) (*Config, error) {
	// Load .env files first (before Viper env binding)
	loadEnvFiles()
	viper.Reset()

	// Set up Viper for environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	_ = viper.BindEnv("output", "OUTPUT")

	// Bind common API keys
	bindAPIKeys()

	// Try to read config file if it exists
	selectedFile := configFile
	if selectedFile == "" {
		selectedFile = viper.GetString("config")
	}
	explicitFile := selectedFile != ""
	if explicitFile {
		viper.SetConfigFile(selectedFile)
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
	} else if explicitFile {
		return nil, fmt.Errorf("read config file %q: %w", selectedFile, err)
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
		CatalogPath:                   viper.GetString("catalog_path"),
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

// loadEnvFiles loads environment variables from .env files.
func loadEnvFiles() {
	// Try to load .env files in order of precedence
	// .env.local overrides .env
	envFiles := []string{
		".env",
		".env.local",
	}

	for _, envFile := range envFiles {
		_ = godotenv.Load(envFile)
	}
}

// bindAPIKeys explicitly binds common API key environment variables to Viper.
func bindAPIKeys() {
	// Common API keys that might be in .env files
	apiKeys := []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GOOGLE_API_KEY",
		"GROQ_API_KEY",
		"GEMINI_API_KEY",
		"CLAUDE_API_KEY",
		"AZURE_API_KEY",
		"COHERE_API_KEY",
		"HUGGINGFACE_API_KEY",
		"REPLICATE_API_KEY",
		"TOGETHER_API_KEY",
		"PERPLEXITY_API_KEY",
		"DEEPSEEK_API_KEY",
		"CEREBRAS_API_KEY",
		"MOONSHOT_API_KEY",
	}

	for _, key := range apiKeys {
		if err := viper.BindEnv(key); err != nil {
			// Log warning but continue - this isn't critical
			fmt.Fprintf(os.Stderr, "Warning: failed to bind environment variable %s: %v\n", key, err)
		}
	}
}

// getEnvOrDefault returns the environment variable value or the default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
