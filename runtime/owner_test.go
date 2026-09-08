package runtime

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersistentRuntimeRecordsDirectoryOwner(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	connected := openTestRuntime(t, WithStateDirectory(directory))
	path := filepath.Join(directory, "owner.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record["schema_version"] != float64(1) || record["product"] != "starmap" || record["deployment"] != "local" || record["instance"] != "default" {
		t.Fatalf("unexpected ownership record: %v", record)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	_ = openTestRuntime(t, WithStateDirectory(directory))
	repeated, err := os.ReadFile(path)
	if err != nil || string(raw) != string(repeated) {
		t.Fatal("restart changed directory ownership")
	}
}

func TestPersistentRuntimeRefusesInvalidOwnerBeforeStateWrites(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	marker := filepath.Join(directory, "owner.json")
	raw := []byte(`{"schema_version":99,"product":"starport","deployment":"production","instance":"other"}`)
	if err := os.WriteFile(marker, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
	if connected != nil {
		_ = connected.Close()
	}
	if err == nil {
		t.Fatal("runtime accepted an incompatible directory owner")
	}
	if _, err := os.Stat(filepath.Join(directory, layerDirectoryName)); !os.IsNotExist(err) {
		t.Fatal("owner refusal wrote runtime layers")
	}
	after, err := os.ReadFile(marker)
	if err != nil || string(raw) != string(after) {
		t.Fatal("owner refusal changed the record")
	}
}

func TestRuntimeOwnerChangesRequireMigration(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	initial := openTestRuntime(t, WithStateDirectory(directory))
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	ownerPath := filepath.Join(directory, ownerRecordName)
	original, err := os.ReadFile(ownerPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, owner := range map[string]DirectoryOwner{
		"product":    {Product: "starport", Deployment: "local", Instance: "default"},
		"deployment": {Product: "starmap", Deployment: "production", Instance: "default"},
		"instance":   {Product: "starmap", Deployment: "local", Instance: "replica-b"},
	} {
		t.Run(name, func(t *testing.T) {
			connected, err := Open(t.Context(), WithStateDirectory(directory), WithDirectoryOwner(owner), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
			if connected != nil {
				_ = connected.Close()
			}
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("owner replacement = %v, want migration conflict", err)
			}
			after, err := os.ReadFile(ownerPath)
			if err != nil || string(after) != string(original) {
				t.Fatal("owner conflict changed the record")
			}
		})
	}
	_ = openTestRuntime(t, WithStateDirectory(directory))
}

func TestDirectoryOwnerValidationIsPassive(t *testing.T) {
	t.Parallel()
	for name, owner := range map[string]DirectoryOwner{
		"unknown_product":    {Product: "other", Deployment: "local", Instance: "default"},
		"empty_deployment":   {Product: "starmap", Instance: "default"},
		"control_deployment": {Product: "starmap", Deployment: "private\x1bvalue", Instance: "default"},
		"invalid_utf8":       {Product: "starmap", Deployment: string([]byte{255}), Instance: "default"},
		"path_instance":      {Product: "starmap", Deployment: "local", Instance: "../other"},
	} {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "unused")
			connected, err := Open(t.Context(), WithStateDirectory(directory), WithDirectoryOwner(owner), WithCatalogSource("embedded"))
			if connected != nil {
				_ = connected.Close()
			}
			if err == nil {
				t.Fatal("invalid ownership identity was accepted")
			}
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatal("invalid owner configuration created state")
			}
		})
	}
}

func TestSchedulerIdentityDoesNotChangeWithListenAddress(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithListenAddress("127.0.0.1:8080"))
	identity := first.Status().InstanceIdentity
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := openTestRuntime(t, WithStateDirectory(directory), WithListenAddress("127.0.0.1:9090"))
	if second.Status().InstanceIdentity != identity {
		t.Fatal("listen address changed persistent instance identity")
	}
	if _, err := os.Stat(filepath.Join(directory, instanceSeedFileName)); err != nil {
		t.Fatal("seed is absent from its canonical runtime location")
	}
}

type ownerProbeStore struct {
	*storage.Memory
	reads int
}

func (s *ownerProbeStore) Current(ctx context.Context) (catalogs.Generation, error) {
	s.reads++
	return s.Memory.Current(ctx)
}

func TestInvalidInstanceSeedRefusesBeforeCatalogAccess(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"empty", "oversized", "uppercase", "missing", "legacy", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			first := openTestRuntime(t, WithStateDirectory(directory))
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, instanceSeedFileName)
			switch mode {
			case "empty":
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				if err := os.WriteFile(path, []byte(strings.Repeat("a", 4096)), 0o600); err != nil {
					t.Fatal(err)
				}
			case "uppercase":
				if err := os.WriteFile(path, []byte(strings.Repeat("A", 32)), 0o600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "legacy":
				if err := os.Rename(path, filepath.Join(directory, layerDirectoryName, instanceSeedFileName)); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(directory, ownerRecordName), path); err != nil {
					t.Skipf("native symlinks unavailable: %v", err)
				}
			}
			probe := &ownerProbeStore{Memory: storage.NewMemory()}
			connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithClientOptions(starmap.WithCatalogStore(probe)))
			if connected != nil {
				_ = connected.Close()
			}
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("invalid seed = %v, want recovery conflict", err)
			}
			if probe.reads != 0 {
				t.Fatal("invalid seed reached the catalog store")
			}
		})
	}
}

func TestOwnerRecordRecoversBeforeSeedInitialization(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestOwnerRecordCrashChild$")
	command.Env = append(os.Environ(), "STARMAP_OWNER_CRASH_TEST="+directory)
	err = command.Run()
	var exit *exec.ExitError
	if !stderrors.As(err, &exit) || exit.ExitCode() != 86 {
		t.Fatalf("owner crash exit = %v", err)
	}
	original, err := os.ReadFile(filepath.Join(directory, ownerRecordName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, instanceSeedFileName)); !os.IsNotExist(err) {
		t.Fatal("crash fixture created a seed")
	}
	connected := openTestRuntime(t, WithStateDirectory(directory))
	if connected.Status().InstanceIdentity == "" {
		t.Fatal("recovery established no identity")
	}
	after, err := os.ReadFile(filepath.Join(directory, ownerRecordName))
	if err != nil || string(after) != string(original) {
		t.Fatal("recovery changed the recorded owner")
	}
}

func TestOwnerRecordCrashChild(t *testing.T) {
	directory := os.Getenv("STARMAP_OWNER_CRASH_TEST")
	if directory == "" {
		return
	}
	lock, err := acquireDirectory(t.Context(), directory)
	if err != nil || lock == nil {
		t.Fatalf("directory lock = %v", err)
	}
	if err := bindDirectoryOwner(t.Context(), directory, DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}, ""); err != nil {
		t.Fatal(err)
	}
	os.Exit(86)
}

func TestSchedulerOverrideIsBoundToDirectoryOwner(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithSchedulerIdentity("owned-a"))
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	changed, err := Open(t.Context(), WithStateDirectory(directory), WithSchedulerIdentity("owned-b"), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
	if changed != nil {
		_ = changed.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("changed scheduler identity = %v, want ownership conflict", err)
	}
	recovered := openTestRuntime(t, WithStateDirectory(directory), WithSchedulerIdentity("owned-a"))
	if recovered.Status().InstanceIdentity != "owned-a" {
		t.Fatal("restart did not retain the explicit identity")
	}
}
