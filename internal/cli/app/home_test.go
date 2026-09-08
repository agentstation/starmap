package app

import (
	"path/filepath"
	"testing"
)

func setTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	roaming, local := "", ""
	if home != "" {
		roaming = filepath.Join(home, "AppData", "Roaming")
		local = filepath.Join(home, "AppData", "Local")
	}
	t.Setenv("APPDATA", roaming)
	t.Setenv("LOCALAPPDATA", local)
}
