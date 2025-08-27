package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootPersistentFlagConfigFile string
	rootPersistentFlagVerbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "starmap",
	Short: "AI Model Catalog CLI",
	Long: `Starmap is a comprehensive AI model catalog system that provides
information about AI models, their capabilities, and providers.

It includes an embedded catalog of models that can be accessed offline,
as well as the ability to fetch live model information from provider APIs
when API keys are configured.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	// Set up context with signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), 
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	
	// Pass context to root command
	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&rootPersistentFlagConfigFile, "config", "", "config file (default is $HOME/.starmap.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&rootPersistentFlagVerbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	if err := viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		panic(fmt.Sprintf("Failed to bind verbose flag: %v", err))
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if rootPersistentFlagConfigFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(rootPersistentFlagConfigFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".starmap" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".starmap")
	}

	// Load .env files first (before Viper env binding)
	loadEnvFiles()

	// Set up environment variable handling
	viper.AutomaticEnv() // Read in environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Explicitly bind common API key environment variables
	// This ensures Viper can access them even if not referenced in config
	bindAPIKeys()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil && rootPersistentFlagVerbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// loadEnvFiles loads environment variables from .env files
func loadEnvFiles() {
	// Try to load .env files in order of precedence
	// .env.local overrides .env
	envFiles := []string{
		".env",
		".env.local",
	}

	for _, envFile := range envFiles {
		loadEnvFile(envFile)
	}
}

// loadEnvFile loads a single .env file using godotenv
func loadEnvFile(filename string) {
	// Use godotenv to load the file into the environment
	if err := godotenv.Load(filename); err == nil {
		// File loaded successfully
		if rootPersistentFlagVerbose {
			fmt.Fprintf(os.Stderr, "Loaded %s\n", filename)
		}
	}
}

// bindAPIKeys explicitly binds common API key environment variables to Viper
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
	}

	for _, key := range apiKeys {
		if err := viper.BindEnv(key); err != nil {
			// Log warning but continue - this isn't critical
			fmt.Printf("Warning: failed to bind environment variable %s: %v\n", key, err)
		}
	}
}
