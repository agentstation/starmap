// Package notify provides context detection for smart hint generation.
package notify

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/agentstation/starmap/internal/cmd/hints"
)

// ContextBuilder helps build hint contexts from command execution.
type ContextBuilder struct {
	context hints.Context
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		context: hints.Context{
			Environment: detectEnvironment(),
			UserState:   detectUserState(),
		},
	}
}

// FromCommand configures the context from a Cobra command.
func (cb *ContextBuilder) FromCommand(cmd *cobra.Command, args []string) *ContextBuilder {
	cb.context.Command = cmd.Name()
	cb.context.Args = args
	
	// Extract subcommand if present
	if cmd.Parent() != nil && cmd.Parent().Name() != "starmap" {
		cb.context.Subcommand = cmd.Name()
		cb.context.Command = cmd.Parent().Name()
	}
	
	// Extract flags
	cb.context.Flags = make(map[string]string)
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			cb.context.Flags[flag.Name] = flag.Value.String()
		}
	})
	
	return cb
}

// WithSuccess sets the operation success status.
func (cb *ContextBuilder) WithSuccess(succeeded bool) *ContextBuilder {
	cb.context.Succeeded = succeeded
	return cb
}

// WithError sets the error type for failed operations.
func (cb *ContextBuilder) WithError(errorType string) *ContextBuilder {
	cb.context.ErrorType = errorType
	cb.context.Succeeded = false
	return cb
}

// WithUserState updates the user state information.
func (cb *ContextBuilder) WithUserState(userState hints.UserState) *ContextBuilder {
	cb.context.UserState = userState
	return cb
}

// Build returns the constructed context.
func (cb *ContextBuilder) Build() hints.Context {
	return cb.context
}

// detectEnvironment detects the current runtime environment.
func detectEnvironment() hints.Environment {
	env := hints.Environment{
		OS:         runtime.GOOS,
		IsTerminal: isTerminal(os.Stdout),
		IsCI:       isCI(),
	}
	
	// Detect shell
	if shell := os.Getenv("SHELL"); shell != "" {
		env.Shell = filepath.Base(shell)
	}
	
	// Working directory
	if wd, err := os.Getwd(); err == nil {
		env.WorkingDir = wd
		
		// Check if in git repository
		env.IsGitRepo = isGitRepository(wd)
	}
	
	return env
}

// detectUserState detects the current user configuration state.
func detectUserState() hints.UserState {
	state := hints.UserState{
		AuthProviders: detectConfiguredProviders(),
		HasConfig:     hasConfigFile(),
		IsFirstRun:    isFirstRun(),
	}
	
	// Detect preferred output format from environment or config
	if format := os.Getenv("STARMAP_OUTPUT_FORMAT"); format != "" {
		state.ConfiguredOutput = format
	}
	
	return state
}

// detectConfiguredProviders detects which authentication providers are configured.
func detectConfiguredProviders() []string {
	var providers []string
	
	// Check common API key environment variables
	apiKeys := map[string]string{
		"OPENAI_API_KEY":     "openai",
		"ANTHROPIC_API_KEY":  "anthropic",
		"GOOGLE_API_KEY":     "google-ai-studio",
		"GROQ_API_KEY":       "groq",
		"DEEPSEEK_API_KEY":   "deepseek",
		"CEREBRAS_API_KEY":   "cerebras",
	}
	
	for envVar, provider := range apiKeys {
		if os.Getenv(envVar) != "" {
			providers = append(providers, provider)
		}
	}
	
	return providers
}

// hasConfigFile checks if the user has a starmap configuration file.
func hasConfigFile() bool {
	// Check common config locations
	configPaths := []string{
		".starmap.yaml",
		".starmap.yml",
	}
	
	if home, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths,
			filepath.Join(home, ".starmap.yaml"),
			filepath.Join(home, ".starmap.yml"),
		)
	}
	
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	
	return false
}

// isFirstRun detects if this is the user's first time running starmap.
func isFirstRun() bool {
	// Check for any indication of previous usage
	indicators := []string{
		".starmap",
		".starmap.yaml",
		".starmap.yml",
	}
	
	// Check current directory
	for _, indicator := range indicators {
		if _, err := os.Stat(indicator); err == nil {
			return false
		}
	}
	
	// Check home directory
	if home, err := os.UserHomeDir(); err == nil {
		for _, indicator := range indicators {
			path := filepath.Join(home, indicator)
			if _, err := os.Stat(path); err == nil {
				return false
			}
		}
	}
	
	// If no indicators found, likely first run
	return true
}

// isGitRepository checks if the given directory is within a git repository.
func isGitRepository(dir string) bool {
	// Walk up the directory tree looking for .git
	current := dir
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return true
		}
		
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}
	
	return false
}

// CommonContexts provides pre-built contexts for common scenarios.
type CommonContexts struct{}

// AuthStatus creates a context for auth status commands.
func (CommonContexts) AuthStatus(succeeded bool, providersConfigured int) hints.Context {
	ctx := NewContextBuilder().Build()
	ctx.Command = "auth"
	ctx.Subcommand = "status"
	ctx.Succeeded = succeeded
	
	// Update provider count
	if providersConfigured == 0 {
		ctx.UserState.AuthProviders = nil
	} else {
		// Populate with actual detected providers
		ctx.UserState.AuthProviders = detectConfiguredProviders()
	}
	
	return ctx
}

// AuthVerify creates a context for auth verification commands.
func (CommonContexts) AuthVerify(succeeded bool, errorType string) hints.Context {
	ctx := NewContextBuilder().Build()
	ctx.Command = "auth"
	ctx.Subcommand = "verify"
	ctx.Succeeded = succeeded
	
	if !succeeded && errorType != "" {
		ctx.ErrorType = errorType
	}
	
	return ctx
}

// Validation creates a context for validation commands.
func (CommonContexts) Validation(subcommand string, succeeded bool, errorType string) hints.Context {
	ctx := NewContextBuilder().Build()
	ctx.Command = "validate"
	ctx.Subcommand = subcommand
	ctx.Succeeded = succeeded
	
	if !succeeded && errorType != "" {
		ctx.ErrorType = errorType
	}
	
	return ctx
}

// Command creates a context for generic command execution.
func (CommonContexts) Command(command, subcommand string, succeeded bool, errorType string) hints.Context {
	ctx := NewContextBuilder().Build()
	ctx.Command = command
	ctx.Subcommand = subcommand
	ctx.Succeeded = succeeded
	
	if !succeeded && errorType != "" {
		ctx.ErrorType = errorType
	}
	
	return ctx
}

// Global contexts instance for convenience
var Contexts CommonContexts