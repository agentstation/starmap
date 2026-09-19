//go:build linux || darwin

package productpaths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func TestConfigurationInputSeparatesServiceAndDotenvAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")
	if err := os.WriteFile(path, []byte("private setting"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	input := productpaths.ConfigurationInput{Path: path, MaxBytes: 64}
	if _, err := productpaths.ReadConfiguration(t.Context(), input); err == nil {
		t.Fatal("default configuration policy accepted shared reads")
	}
	input.AccessPolicy, input.Explicit = policy.ServiceManaged, true
	if _, err := productpaths.ReadConfiguration(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := productpaths.ReadDotenv(t.Context(), path, 64); err == nil {
		t.Fatal("dotenv inherited the primary configuration exception")
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	if _, err := productpaths.ReadConfiguration(t.Context(), input); err == nil {
		t.Fatal("service-managed configuration accepted group writes")
	}
}
