package app

import (
	"os"
	"path/filepath"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/runtime"
)

func legacyMigrationFixture(t *testing.T) (*Config, runtime.DirectoryMigrationRequest) {
	t.Helper()
	clearCatalogEnvironment(t)
	for _, setting := range pathSettings() {
		t.Setenv(setting.name, "")
		if err := os.Unsetenv(setting.name); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	for name, value := range map[string]string{
		"HOME": home, "USERPROFILE": home, "APPDATA": filepath.Join(home, "roaming"), "LOCALAPPDATA": filepath.Join(home, "local"),
		"XDG_CONFIG_HOME": filepath.Join(home, "config"), "XDG_DATA_HOME": filepath.Join(home, "data"),
		"XDG_STATE_HOME": filepath.Join(home, "state"), "XDG_CACHE_HOME": filepath.Join(home, "cache"),
	} {
		t.Setenv(name, value)
	}
	config := &Config{CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}}
	a, err := New("test", "test", "test", "test", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Runtime.Origin != "platform-default" {
		t.Fatalf("fixture runtime origin = %s", paths.Runtime.Origin)
	}
	old := filepath.Join(home, ".starmap", "state", "runtime")
	source, err := runtime.Open(t.Context(), runtime.WithStateDirectory(old), runtime.WithCatalogSource("embedded"), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false))
	if err != nil {
		t.Fatal(err)
	}
	identity := source.Status().InstanceIdentity
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	return config, runtime.DirectoryMigrationRequest{OperationID: "legacy-root", SourceDirectory: old, TargetDirectory: paths.Runtime.Path,
		SourceIdentity: identity, JournalRoot: filepath.Join(home, "journal"), Owner: runtime.DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}}
}

func TestDefaultRuntimeRootAcceptsCompletedLegacyMigration(t *testing.T) {
	config, request := legacyMigrationFixture(t)
	if _, err := runtime.PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	selected, err := runtime.Open(t.Context(), runtime.WithStateDirectory(request.TargetDirectory), runtime.WithSchedulerIdentity(request.SourceIdentity), runtime.WithCatalogSource("embedded"), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selected.CompleteDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	config.CatalogValues[catalogconfig.SchedulerIdentity] = request.SourceIdentity
	a, err := New("test", "test", "test", "test", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := a.Runtime(t.Context())
	if err != nil {
		t.Fatalf("completed migration still blocks default startup: %v", err)
	}
	if opened.Status().InstanceIdentity != request.SourceIdentity {
		t.Fatal("legacy acknowledgement changed runtime identity")
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(request.SourceDirectory, "instance-seed")); err != nil {
		t.Fatal("legacy acknowledgement removed the old seed")
	}
}

func TestDefaultRuntimeRootRefusesUnprovenLegacyMigration(t *testing.T) {
	for _, change := range []string{"published-only", "changed-source", "wrong-identity", "another-root"} {
		t.Run(change, func(t *testing.T) {
			config, request := legacyMigrationFixture(t)
			if _, err := runtime.PublishDirectoryMigration(t.Context(), request); err != nil {
				t.Fatal(err)
			}
			if change != "published-only" {
				selected, err := runtime.Open(t.Context(), runtime.WithStateDirectory(request.TargetDirectory), runtime.WithSchedulerIdentity(request.SourceIdentity), runtime.WithCatalogSource("embedded"), runtime.WithSourcePollInterval(0), runtime.WithAcquisitionEnabled(false))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := selected.CompleteDirectoryMigration(t.Context(), request); err != nil {
					t.Fatal(err)
				}
				if err := selected.Close(); err != nil {
					t.Fatal(err)
				}
			}
			config.CatalogValues[catalogconfig.SchedulerIdentity] = request.SourceIdentity
			switch change {
			case "changed-source":
				if err := os.WriteFile(filepath.Join(request.SourceDirectory, "untracked-file"), []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "wrong-identity":
				config.CatalogValues[catalogconfig.SchedulerIdentity] = "another-identity"
			case "another-root":
				if err := os.WriteFile(filepath.Join(filepath.Dir(request.SourceDirectory), "instance-seed"), []byte("another root"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			a, err := New("test", "test", "test", "test", WithConfig(config))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.Runtime(t.Context()); err == nil {
				_ = a.closeRuntime()
				t.Fatal("unproven legacy migration allowed startup")
			}
		})
	}
}
