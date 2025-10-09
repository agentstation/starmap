package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"github.com/agentstation/starmap/pkg/constants"
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
	LocalPath           string
	UseEmbeddedCatalog  bool
	AutoUpdatesEnabled  bool
	AutoUpdateInterval  time.Duration
	RemoteServerURL     string
	RemoteServerAPIKey  string
	RemoteServerOnly    bool

	// Logging configuration
	LogLevel  string
	LogFormat string
	LogOutput string
}

// LoadConfig loads configuration from all sources in order of precedence:
// 1. Command-line flags (handled by cobra)
// 2. Environment variables
// 3. .env files
// 4. Config file (~/.starmap.yaml)
// 5. Defaults
func LoadConfig() (*Config, error) {
	// Load .env files first (before Viper env binding)
	loadEnvFiles()

	// Set up Viper for environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Bind common API keys
	bindAPIKeys()

	// Try to read config file if it exists
	configFile := viper.GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Search for config in standard locations
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.AddConfigPath(".")
			viper.SetConfigType("yaml")
			viper.SetConfigName(".starmap")
		}
	}

	// Read config file (ignore error if not found)
	_ = viper.ReadInConfig()

	// Build config from viper
	config := &Config{
		// Global flags (may be overridden by cobra flags later)
		Verbose: viper.GetBool("verbose"),
		Quiet:   viper.GetBool("quiet"),
		NoColor: viper.GetBool("no-color"),
		Output:  viper.GetString("output"),

		// Config file
		ConfigFile: viper.ConfigFileUsed(),

		// Starmap configuration
		LocalPath:          viper.GetString("local_path"),
		UseEmbeddedCatalog: viper.GetBool("use_embedded_catalog"),
		AutoUpdatesEnabled: viper.GetBool("auto_updates_enabled"),
		AutoUpdateInterval: viper.GetDuration("auto_update_interval"),
		RemoteServerURL:    viper.GetString("remote_server_url"),
		RemoteServerAPIKey: viper.GetString("remote_server_api_key"),
		RemoteServerOnly:   viper.GetBool("remote_server_only"),

		// Logging configuration
		LogLevel:  getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat: getEnvOrDefault("LOG_FORMAT", "auto"),
		LogOutput: getEnvOrDefault("LOG_OUTPUT", "stderr"),
	}

	// Set defaults
	if config.AutoUpdateInterval == 0 {
		config.AutoUpdateInterval = constants.DefaultUpdateInterval
	}

	return config, nil
}

// UpdateFromFlags updates config values from parsed command flags.
// This should be called after cobra parses flags to ensure flag
// values take precedence over config file and env vars.
func (c *Config) UpdateFromFlags(verbose, quiet, noColor bool, output string) {
	c.Verbose = verbose
	c.Quiet = quiet
	c.NoColor = noColor
	if output != "" {
		c.Output = output
	}
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
