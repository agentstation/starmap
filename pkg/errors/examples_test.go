package errors_test

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// Example demonstrates basic error creation and checking.
func Example() {
	// Create a not found error
	err := &errors.NotFoundError{
		Resource: "model",
		ID:       "gpt-5",
	}

	// Check error type
	if errors.IsNotFound(err) {
		fmt.Println("Resource not found")
	}

	// Output: Resource not found
}

// Example_aPIError demonstrates API error handling.
func Example_aPIError() {
	// Simulate an API error
	err := &errors.APIError{
		Provider:   "openai",
		Endpoint:   "https://api.openai.com/v1/models",
		StatusCode: 429,
		Message:    "Rate limit exceeded",
	}

	// Check and handle specific error types
	switch err.StatusCode {
	case 429:
		fmt.Println("Rate limited - retry later")
	case 401:
		fmt.Println("Authentication failed")
	case 500:
		fmt.Println("Server error")
	}

	// Output: Rate limited - retry later
}

// Example_authenticationError shows authentication error handling.
func Example_authenticationError() {
	// Create authentication error
	err := &errors.AuthenticationError{
		Provider: "anthropic",
		Message:  "API key not configured",
	}

	// Auth error is already typed
	fmt.Printf("Auth failed for %s: %s\n",
		err.Provider, err.Message)

	// Output: Auth failed for anthropic: API key not configured
}

// Example_rateLimitError demonstrates rate limit handling with retry.
func Example_rateLimitError() {
	// Create API error for rate limiting
	err := &errors.APIError{
		Provider:   "openai",
		StatusCode: 429,
		Message:    "Rate limit exceeded. Try again in 30 seconds.",
	}

	// Handle rate limit
	if err.StatusCode == 429 {
		fmt.Printf("Rate limited: %s\n", err.Message)
	}

	// Output: Rate limited: Rate limit exceeded. Try again in 30 seconds.
}

// Example_errorWrapping demonstrates error wrapping patterns.
func Example_errorWrapping() {
	// Original error
	originalErr := fmt.Errorf("connection refused")

	// Wrap with IO error
	ioErr := errors.WrapIO("connect", "api.openai.com", originalErr)

	// Wrap with API error
	_ = &errors.APIError{
		Provider:   "openai",
		Endpoint:   "https://api.openai.com/v1/models",
		StatusCode: 0,
		Message:    "Failed to connect",
		Err:        ioErr,
	}

	// API error type is already known
	fmt.Println("API error occurred")

	// Output: API error occurred
}

// Example_validationError shows input validation errors.
func Example_validationError() {
	// Validate input
	apiKey := ""
	if apiKey == "" {
		err := &errors.ValidationError{
			Field:   "api_key",
			Value:   apiKey,
			Message: "API key cannot be empty",
		}
		fmt.Println(err.Error())
	}

	// Output: validation failed for field api_key: API key cannot be empty
}

// Example_processError demonstrates subprocess error handling.
func Example_processError() {
	// Create process error
	err := &errors.ProcessError{
		Operation: "git clone",
		Command:   "git clone https://github.com/repo.git",
		Output:    "fatal: repository not found",
		ExitCode:  128,
	}

	// Handle process errors
	fmt.Printf("Command failed with exit code %d\n", err.ExitCode)
	if err.ExitCode == 128 {
		fmt.Println("Git configuration error")
	}

	// Output:
	// Command failed with exit code 128
	// Git configuration error
}

// Example_errorRecovery demonstrates error recovery strategies.
func Example_errorRecovery() {
	// Retry strategy for rate limits
	var attemptRequest func() error
	attemptRequest = func() error {
		// Simulate API call
		return &errors.APIError{
			Provider:   "openai",
			StatusCode: 429,
			Message:    "Rate limit: 3 per second",
		}
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := attemptRequest()

		if apiErr, ok := err.(*errors.APIError); ok && apiErr.StatusCode == 429 {
			fmt.Printf("Attempt %d: Rate limited, retrying...\n", i+1)
			time.Sleep(time.Second) // Simple backoff
			continue
		}

		if err != nil {
			log.Fatal(err)
		}

		break
	}
}

// Example_errorChaining shows chained error handling.
func Example_errorChaining() {
	// Create a chain of errors
	baseErr := &errors.NotFoundError{
		Resource: "file",
		ID:       "config.json",
	}

	parseErr := &errors.ParseError{
		Format:  "json",
		File:    "config.json",
		Message: "Failed to parse config",
		Err:     baseErr,
	}

	// Check through the chain using standard library
	if parseErr.Err != nil {
		if _, ok := parseErr.Err.(*errors.NotFoundError); ok {
			fmt.Println("File not found in parse chain")
		}
	}

	// Output: File not found in parse chain
}

// Example_hTTPStatusMapping maps HTTP codes to error types.
func Example_hTTPStatusMapping() {
	// Map HTTP status to appropriate error
	mapHTTPError := func(status int, provider string) error {
		switch status {
		case http.StatusNotFound:
			return &errors.NotFoundError{
				Resource: "endpoint",
				ID:       provider,
			}
		case http.StatusUnauthorized:
			return &errors.AuthenticationError{
				Provider: provider,
				Message:  "Invalid credentials",
			}
		case http.StatusTooManyRequests:
			return &errors.APIError{
				Provider:   provider,
				StatusCode: 429,
				Message:    "Rate limit exceeded",
			}
		default:
			return &errors.APIError{
				Provider:   provider,
				StatusCode: status,
				Message:    http.StatusText(status),
			}
		}
	}

	err := mapHTTPError(401, "openai")
	if _, ok := err.(*errors.AuthenticationError); ok {
		fmt.Println("Authentication required")
	}

	// Output: Authentication required
}
