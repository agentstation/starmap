package main

import (
	"errors"
	"fmt"
	"log"
)

// This example demonstrates how to handle multiple errors returned by our concurrent functions
func main() {
	// Example of how errors.Join() works with multiple errors
	err1 := errors.New("first error from source A")
	err2 := errors.New("second error from source B")
	err3 := errors.New("third error from source C")

	// Join all errors together (this is what our functions now do)
	combinedErr := errors.Join(err1, err2, err3)

	fmt.Println("Combined error message:")
	fmt.Println(combinedErr.Error())
	fmt.Println()

	// You can unwrap individual errors
	fmt.Println("Individual errors:")
	for _, err := range []error{err1, err2, err3} {
		if errors.Is(combinedErr, err) {
			fmt.Printf("- Found: %v\n", err)
		}
	}
	fmt.Println()

	// Example of error handling in calling code
	if err := simulateSourceOperations(); err != nil {
		log.Printf("Operation failed with errors: %v", err)

		// You can handle specific errors if needed
		if errors.Is(err, ErrSourceTimeout) {
			log.Println("At least one source timed out")
		}
		if errors.Is(err, ErrSourceUnavailable) {
			log.Println("At least one source was unavailable")
		}
	}
}

// Simulated error types
var (
	ErrSourceTimeout     = errors.New("source timeout")
	ErrSourceUnavailable = errors.New("source unavailable")
)

// Example function that returns multiple errors (similar to our setup/fetch/cleanup functions)
func simulateSourceOperations() error {
	var errs []error

	// Simulate errors from different sources
	errs = append(errs, fmt.Errorf("source 'providers': %w", ErrSourceTimeout))
	errs = append(errs, fmt.Errorf("source 'models.dev': %w", ErrSourceUnavailable))

	// Return all errors joined together, or nil if no errors
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
