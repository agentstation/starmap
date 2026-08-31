package auth

import (
	"fmt"

	"github.com/spf13/cobra"
)

// mustGetBool retrieves a boolean flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetBool(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}

// mustGetString retrieves a string flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}
