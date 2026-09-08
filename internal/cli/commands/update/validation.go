package update

import (
	"fmt"
	"strings"
)

const (
	affirmativeShort = "y"
	affirmativeLong  = "yes"
)

// ConfirmChanges asks the user to confirm applying changes.
// Returns true if the user confirms, false if cancelled.
func ConfirmChanges() (bool, error) {
	fmt.Printf("Apply these changes? (y/N): ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		response = "n"
	}
	response = strings.ToLower(strings.TrimSpace(response))

	if response != affirmativeShort && response != affirmativeLong {
		fmt.Println("Update cancelled")
		return false, nil
	}

	return true, nil
}
