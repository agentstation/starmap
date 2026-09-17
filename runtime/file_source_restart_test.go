package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestFileSourceRestartRetainsReceiptAndAcceptsChangedPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	write := func(model string) {
		t.Helper()
		if err := os.WriteFile(path, testCatalogPayload(t, "file-provider", model, "File Model"), constants.SecureFilePermissions); err != nil {
			t.Fatal(err)
		}
	}
	write("first")
	directory := privateRuntimeDirectory(t)
	store := storage.NewMemory()
	open := func() *Runtime {
		return openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("file"),
			WithSourceURL(path), WithSourceStartupPolicy("require_source"),
			WithClientOptions(starmap.WithCatalogStore(store)))
	}
	first := open()
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted.Manifest.SourceObservations) == 0 {
		t.Fatal("file baseline has no source receipt")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := open()
	restarted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Manifest.GenerationID != accepted.Manifest.GenerationID ||
		restarted.Manifest.Payload.Checksum != accepted.Manifest.Payload.Checksum ||
		!reflect.DeepEqual(restarted.Manifest.SourceObservations, accepted.Manifest.SourceObservations) {
		t.Fatal("unchanged file acquired a new generation or receipt after restart")
	}
	write("second")
	report, err := second.RefreshSource(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Changed || !report.Published {
		t.Fatalf("changed payload report = %+v", report)
	}
	changed, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if changed.Manifest.GenerationID == accepted.Manifest.GenerationID || changed.Manifest.Payload.Checksum == accepted.Manifest.Payload.Checksum {
		t.Fatal("changed file retained the prior generation")
	}
	provider, err := second.Catalog().Provider("file-provider")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Models["second"] == nil || provider.Models["first"] != nil {
		t.Fatal("replacement baseline did not replace file models")
	}
}

func TestFileSourceRetainsPendingChangeUntilRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, testCatalogPayload(t, "file-provider", "first", "First"), constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("file"), WithSourceURL(path),
		WithSourceStartupPolicy("require_source"), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	accepted := connected.State()
	if err := os.WriteFile(path, testCatalogPayload(t, "file-provider", "replacement", "Replacement"), constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(true)
	if _, err := connected.RefreshSource(t.Context()); err == nil {
		t.Fatal("rejected commit succeeded")
	}
	if connected.State().GenerationID != accepted.GenerationID {
		t.Fatal("failed commit changed the active generation")
	}
	store.reject.Store(false)
	report, err := connected.RefreshSource(t.Context())
	if !pkgerrors.IsConflict(err) || report.Health == HealthOK || report.Published {
		t.Fatalf("pending file change must require journal recovery: report=%+v error=%v", report, err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	connected = openTestRuntime(t, options...)
	if connected.State().GenerationID == accepted.GenerationID {
		t.Fatal("startup recovery lost the uncommitted file change")
	}
	provider, err := connected.Catalog().Provider("file-provider")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Models["replacement"] == nil {
		t.Fatal("retry did not activate the replacement model")
	}
}
