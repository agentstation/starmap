package models

import (
	"github.com/spf13/cobra"
)

// mustGetBool retrieves a boolean flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetBool(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}

// mustGetString retrieves a string flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}

// mustGetInt64 retrieves an int64 flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetInt64(cmd *cobra.Command, name string) int64 {
	val, err := cmd.Flags().GetInt64(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}

// mustGetFloat64 retrieves a float64 flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetFloat64(cmd *cobra.Command, name string) float64 {
	val, err := cmd.Flags().GetFloat64(name)
	if err != nil {
		panic("programming error: failed to get flag " + name + ": " + err.Error())
	}
	return val
}
