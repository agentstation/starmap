package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/runtime"
)

func TestRuntimeMigrationCommandPersistsSelectionBeforeCompletion(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
		catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false",
	}}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	source, err := a.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	identity := source.Status().InstanceIdentity
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "replacement")
	journal := filepath.Join(home, "migrations")
	arguments := []string{"--from", paths.Runtime.Path, "--to", target, "--journal-root", journal, "--operation-id", "cli-root-switch", "--source-identity", identity}
	execute := func(phase string, extra ...string) (map[string]any, error) {
		t.Helper()
		application := NewForCommand("test", "test", "test", "test")
		command := application.createRootCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		args := append([]string{"migrate", "runtime", phase, "--output", "json"}, arguments...)
		command.SetArgs(append(args, extra...))
		err := command.ExecuteContext(t.Context())
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatalf("migration output is not JSON: %s", output.String())
		}
		return result, nil
	}
	for _, phase := range []struct{ command, want string }{{"prepare", "prepared"}, {"stage", "verified"}, {"publish", "promoted"}} {
		result, err := execute(phase.command)
		if err != nil || result["phase"] != phase.want {
			t.Fatalf("%s result = %v, %v", phase.command, result, err)
		}
	}
	if _, err := execute("complete", "--state-dir", target, "--scheduler-identity", identity); err == nil {
		t.Fatal("flags alone completed a durable root switch")
	}
	file := filepath.Join(home, "selected.yaml")
	encoded, err := json.Marshal(map[string]any{"state_dir": target, "scheduler_identity": identity,
		"catalog_source": "embedded", "catalog_source_poll_interval": "0s", "catalog_acquisition_enabled": false})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		result, err := execute("complete", "--config", file)
		if err != nil || result["phase"] != "completed" {
			t.Fatalf("complete result = %v, %v", result, err)
		}
	}
	retained, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(retained, encoded) {
		t.Fatal("completion changed the selected configuration")
	}
	// A new process must reopen the target after the completion command releases it.
	loaded, err := loadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := New("test", "test", "test", "test", WithConfig(loaded))
	if err != nil {
		t.Fatal(err)
	}
	running, err := replacement.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if running.Status().InstanceIdentity != identity {
		t.Fatal("CLI migration changed runtime identity")
	}
	if err := replacement.closeRuntime(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeMigrationCommandRejectsInvalidInputBeforeWrites(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	source, target, journal := filepath.Join(home, "source"), filepath.Join(home, "target"), filepath.Join(home, "journal")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(source, "instance-seed")
	if err := os.WriteFile(seed, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, source, format, operation string
		extra                           []string
	}{
		{name: "missing operation", source: source, format: "json"},
		{name: "relative source", source: "relative", format: "json", operation: "operation"},
		{name: "invalid output", source: source, format: "invalid", operation: "operation"},
		{name: "extra argument", source: source, format: "json", operation: "operation", extra: []string{"unexpected"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewForCommand("test", "test", "test", "test")
			command := a.createRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			args := []string{"migrate", "runtime", "prepare", "--from", tc.source, "--to", target, "--journal-root", journal, "--source-identity", "observed-legacy", "--output", tc.format}
			if tc.operation != "" {
				args = append(args, "--operation-id", tc.operation)
			}
			command.SetArgs(append(args, tc.extra...))
			if err := command.ExecuteContext(t.Context()); err == nil {
				t.Fatal("invalid command succeeded")
			}
			for _, path := range []string{target, journal} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("invalid command created migration state")
				}
			}
			files, err := os.ReadDir(source)
			if err != nil || len(files) != 1 || files[0].Name() != "instance-seed" {
				t.Fatal("invalid command changed the source directory")
			}
		})
	}
}

func TestRuntimeMigrationUsesCanonicalJournalAndOwner(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	t.Setenv("STARMAP_DEPLOYMENT_ID", "production")
	t.Setenv("STARMAP_INSTANCE_ID", "catalog-a")
	source := filepath.Join(home, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "instance-seed"), []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.PrepareRuntimeMigration(t.Context(), runtime.DirectoryMigrationRequest{
		SourceDirectory: source, TargetDirectory: filepath.Join(home, "target"), OperationID: "owner-selection", SourceIdentity: "observed-legacy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.JournalDirectory, filepath.Join(home, "state", "migrations")+string(filepath.Separator)) {
		t.Fatalf("journal directory = %s", result.JournalDirectory)
	}
	data, err := os.ReadFile(filepath.Join(result.JournalDirectory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Owner runtime.DirectoryOwner `json:"owner"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Owner != (runtime.DirectoryOwner{Product: "starmap", Deployment: "production", Instance: "catalog-a"}) {
		t.Fatalf("recorded owner = %+v", record.Owner)
	}
}

func TestRuntimeMigrationCompletionRefusesChangedConfigurationBeforeStartup(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	target := filepath.Join(home, "must-stay-absent")
	file := filepath.Join(home, "selected.yaml")
	data, err := json.Marshal(map[string]any{"state_dir": target, "scheduler_identity": "retained-identity", "catalog_source": "embedded"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New("test", "test", "test", "test", WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	changed := append(data, '\n')
	if err := os.WriteFile(file, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	request := runtime.DirectoryMigrationRequest{OperationID: "changed-selection", SourceDirectory: filepath.Join(home, "source"), TargetDirectory: target, SourceIdentity: "retained-identity"}
	if _, err := a.CompleteRuntimeMigration(t.Context(), request); err == nil || err.Error() != "runtime migration selection conflict: configuration file changed after selection" {
		t.Fatalf("changed configuration = %v", err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("refused completion initialized the target")
	}
	retained, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(retained, changed) {
		t.Fatal("refused completion overwrote the changed configuration")
	}
}
