// Package hints provides context-aware hint providers for Starmap CLI.
package hints

import (
	"fmt"
	"strings"
)

const (
	authCommand = "auth"
)

// RegisterStarmapProviders registers all standard Starmap hint providers.
func RegisterStarmapProviders(registry *Registry) {
	// Auth-related hints
	registry.RegisterFunc(authCommand, authHintProvider)

	// Configuration hints
	registry.RegisterFunc("config", configHintProvider)

	// First-run experience hints
	registry.RegisterFunc("onboarding", onboardingHintProvider)

	// Command-specific hints
	registry.RegisterFunc("commands", commandHintProvider)

	// Error recovery hints
	registry.RegisterFunc("errors", errorRecoveryHintProvider)
}

// authHintProvider provides authentication-related hints.
func authHintProvider(ctx Context) []*Hint {
	var hints []*Hint

	// No providers configured
	if len(ctx.UserState.AuthProviders) == 0 {
		if ctx.Command == authCommand && ctx.Subcommand == "status" {
			hints = append(hints, NewCommand(
				"Set up API keys to access AI models",
				"export OPENAI_API_KEY=your-key-here",
			).WithTags("setup", "auth"))
		}
	}

	// Auth status success - suggest verification
	if ctx.Command == authCommand && ctx.Subcommand == "status" && ctx.Succeeded {
		if len(ctx.UserState.AuthProviders) > 0 {
			hints = append(hints, NewCommand(
				"Test that your credentials work",
				"starmap providers auth verify",
			).WithTags("verification", "next-step"))
		}
	}

	// Auth verification failures
	if ctx.Command == authCommand && ctx.Subcommand == "verify" && !ctx.Succeeded {
		hints = append(hints, New(
			"Check that your API keys are valid and not expired",
		).WithTags("troubleshooting", "auth"))

		hints = append(hints, NewCommand(
			"View current authentication status",
			"starmap providers auth status",
		).WithTags("troubleshooting", "auth"))
	}

	return hints
}

// configHintProvider provides configuration-related hints.
func configHintProvider(_ Context) []*Hint {
	var hints []*Hint

	// Only show config hints on first run or when there are actual config issues
	// Don't spam every successful command with generic config info

	return hints
}

// onboardingHintProvider provides first-run experience hints.
func onboardingHintProvider(ctx Context) []*Hint {
	var hints []*Hint

	// Only show onboarding hints during actual first-run scenarios
	// Not on every successful command
	if !ctx.UserState.IsFirstRun {
		return hints
	}

	// Only show if this is actually a help command or no providers configured
	if ctx.Command == "help" || ctx.Command == "version" || len(ctx.UserState.AuthProviders) == 0 {
		hints = append(hints, NewCommand(
			"Start by checking authentication status",
			"starmap providers auth status",
		).WithTags("onboarding", "getting-started"))
	}

	return hints
}

// commandHintProvider provides command-specific hints.
func commandHintProvider(ctx Context) []*Hint {
	var hints []*Hint

	switch ctx.Command {
	case "list":
		if ctx.Subcommand == "models" && ctx.Succeeded {
			hints = append(hints, NewCommand(
				"Get detailed information about a specific model",
				"starmap models <model-name>",
			).WithTags("exploration", "models"))
		}

		if ctx.Subcommand == "providers" && ctx.Succeeded {
			hints = append(hints, NewCommand(
				"View authentication status for providers",
				"starmap providers auth status",
			).WithTags("exploration", "auth"))
		}

	case "update":
		if ctx.Succeeded {
			hints = append(hints, NewCommand(
				"Verify updated provider credentials",
				"starmap providers auth verify",
			).WithTags("verification", "update"))
		}

	case "validate":
		if ctx.Subcommand == "catalog" && !ctx.Succeeded {
			hints = append(hints, New(
				"Catalog validation errors may indicate outdated or corrupted data",
			).WithTags("troubleshooting", "catalog"))

			hints = append(hints, NewCommand(
				"Try updating the catalog",
				"starmap update",
			).WithTags("recovery", "catalog"))
		}

	case "serve":
		if ctx.Succeeded {
			hints = append(hints, New(
				"Press Ctrl+C to stop the server",
			).WithTags("usage", "server"))
		}
	}

	return hints
}

// errorRecoveryHintProvider provides hints for error recovery.
func errorRecoveryHintProvider(ctx Context) []*Hint {
	var hints []*Hint

	if ctx.Succeeded {
		return hints
	}

	switch ctx.ErrorType {
	case "auth_failed":
		hints = append(hints, NewCommand(
			"Check your API key configuration",
			"starmap providers auth status",
		).WithTags("recovery", "auth"))

	case "network_error":
		hints = append(hints, New(
			"Check your internet connection and try again",
		).WithTags("recovery", "network"))

		hints = append(hints, New(
			"Some providers may be experiencing temporary outages",
		).WithTags("recovery", "network"))

	case "rate_limit":
		hints = append(hints, New(
			"Wait a few moments and try again - you've hit a rate limit",
		).WithTags("recovery", "rate-limit"))

	case "permission_denied":
		hints = append(hints, New(
			"Check file permissions in your working directory",
		).WithTags("recovery", "permissions"))

	case "not_found":
		if ctx.Command == "list" {
			hints = append(hints, NewCommand(
				"See all available options",
				fmt.Sprintf("starmap %s", ctx.Command),
			).WithTags("recovery", "discovery"))
		}
	}

	// General troubleshooting hint
	if ctx.ErrorType != "" {
		hints = append(hints, NewCommand(
			"Run with verbose output for more details",
			fmt.Sprintf("%s --verbose", strings.Join(append([]string{ctx.Command}, ctx.Args...), " ")),
		).WithTags("troubleshooting", "debugging"))
	}

	return hints
}
