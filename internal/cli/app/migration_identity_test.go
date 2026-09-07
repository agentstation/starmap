package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/runtime"
)

func TestSchedulerIdentityConfigurationPrecedence(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	directory := filepath.Join(home, "config")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.yaml"), []byte("scheduler_identity: from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dotenv := filepath.Join(home, "explicit.env")
	if err := os.WriteFile(dotenv, []byte("STARMAP_SCHEDULER_IDENTITY=from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, environment, want, origin string
		args                            []string
	}{
		{name: "file", want: "from-file", origin: "configuration-file"},
		{name: "dotenv", args: []string{"--env-file", dotenv}, want: "from-dotenv", origin: "dotenv:" + dotenv},
		{name: "environment", environment: "from-environment", args: []string{"--env-file", dotenv}, want: "from-environment", origin: "environment"},
		{name: "flag", environment: "from-environment", args: []string{"--env-file", dotenv, "--scheduler-identity", "from-flag"}, want: "from-flag", origin: "override-1"},
		{name: "empty flag", environment: "from-environment", args: []string{"--scheduler-identity="}, origin: "override-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STARMAP_SCHEDULER_IDENTITY", tc.environment)
			if tc.environment == "" {
				if err := os.Unsetenv("STARMAP_SCHEDULER_IDENTITY"); err != nil {
					t.Fatal(err)
				}
			}
			a := NewForCommand("test", "test", "test", "test")
			if err := a.Execute(t.Context(), append(tc.args, "version")); err != nil {
				t.Fatal(err)
			}
			paths, err := a.ResolvedPaths()
			if err != nil {
				t.Fatal(err)
			}
			if paths.SchedulerIdentity != tc.want || paths.SchedulerIdentityOrigin != tc.origin {
				t.Fatalf("scheduler selection = %q (%s)", paths.SchedulerIdentity, paths.SchedulerIdentityOrigin)
			}
		})
	}
}

func TestApplicationSelectsPublishedMigrationThroughConfiguration(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	config := &Config{
		CatalogValues: map[string]string{catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false", catalogconfig.SchedulerIdentity: ""},
	}
	a, err := New("test", "test", "test", "test", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	connected, err := a.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	identity := connected.Status().InstanceIdentity
	if identity == "" {
		t.Fatal("Go configuration did not select a derived identity")
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	request := runtime.DirectoryMigrationRequest{
		OperationID: "application-root-switch", SourceDirectory: paths.Runtime.Path,
		TargetDirectory: filepath.Join(home, "replacement-runtime"), JournalRoot: filepath.Join(home, "migration-journal"),
		SourceIdentity: identity, Owner: runtime.DirectoryOwner{Product: "starmap", Deployment: paths.DeploymentID, Instance: paths.InstanceID},
	}
	if _, err := runtime.PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	config.CatalogValues[catalogconfig.SchedulerIdentity] = identity
	config.CatalogValues[catalogconfig.StateDirectory] = request.TargetDirectory
	replacement, err := New("test", "test", "test", "test", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	active, err := replacement.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = replacement.closeRuntime() })
	if active.Status().InstanceIdentity != identity {
		t.Fatal("replacement changed the migrated identity")
	}
	if _, err := active.CompleteDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := replacement.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	// JSON is valid YAML and quotes platform-specific paths without shell expansion.
	for _, override := range []string{identity, "changed", ""} {
		data, err := json.Marshal(map[string]any{
			"scheduler_identity": override,
			"catalog_source":     "embedded", "catalog_source_poll_interval": "0s", "catalog_acquisition_enabled": false, "state_dir": request.TargetDirectory,
		})
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(home, "selected-config.yaml")
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Unsetenv("STARMAP_SCHEDULER_IDENTITY"); err != nil {
			t.Fatal(err)
		}
		loaded, err := loadConfig(file)
		if err != nil {
			t.Fatal(err)
		}
		reopened, err := New("test", "test", "test", "test", WithConfig(loaded))
		if err != nil {
			t.Fatal(err)
		}
		running, err := reopened.Runtime(t.Context())
		if override != identity {
			if err == nil {
				_ = reopened.closeRuntime()
				t.Fatal("configuration changed the bound scheduler identity")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if running.Status().InstanceIdentity != identity {
			t.Fatal("configuration reload lost the migrated identity")
		}
		if err := reopened.closeRuntime(); err != nil {
			t.Fatal(err)
		}
	}
}
