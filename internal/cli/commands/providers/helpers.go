package providers

import (
	"fmt"
	"time"

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

// mustGetDuration retrieves a duration flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetDuration(cmd *cobra.Command, name string) time.Duration {
	val, err := cmd.Flags().GetDuration(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}
