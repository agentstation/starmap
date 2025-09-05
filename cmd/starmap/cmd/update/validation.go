package update

import (
	"fmt"
	"os"
	"strings"
)

// ValidateForceUpdate shows a warning for force updates and asks for confirmation.
// Returns true if the user confirms to proceed, false if cancelled.
func ValidateForceUpdate(isQuiet, autoApprove bool) (bool, error) {
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Force mode will DELETE all existing model files and replace them with fresh API models.\n")
	}

	if autoApprove {
		return true, nil
	}

	fmt.Printf("\nContinue with force update? (y/N): ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		response = "n"
	}
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" && response != "yes" {
		fmt.Println("Force update cancelled")
		return false, nil
	}

	fmt.Println()
	return true, nil
}

// ConfirmChanges asks the user to confirm applying changes.
// Returns true if the user confirms, false if cancelled.
func ConfirmChanges() (bool, error) {
	fmt.Printf("Apply these changes? (y/N): ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		response = "n"
	}
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" && response != "yes" {
		fmt.Println("Update cancelled")
		return false, nil
	}

	return true, nil
}
