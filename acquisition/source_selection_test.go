package acquisition

import (
	stderrors "errors"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestManualSourceSelectionRejectsExcludedReadsBeforePreparation(t *testing.T) {
	transport := &sourcePathTransport{testing: t}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	connected, err := runtime.Open(t.Context(), runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false), runtime.WithAcquisitionSources(), runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	}()
	calls := 0
	syncer, err := NewForRuntime(connected, WithProviderClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) { calls++; return receiptProviderClient{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	before := connected.State().GenerationID
	for _, selected := range []sources.ID{sources.ProvidersID, sources.ModelsDevHTTPID, sources.LocalCatalogID} {
		for _, dry := range []bool{false, true} {
			if _, err := syncer.Sync(t.Context(), pkgsync.WithSources(selected), pkgsync.WithDryRun(dry)); err == nil {
				t.Fatal("excluded source entered manual preparation", selected)
			}
		}
	}
	if calls != 0 || transport.calls != 0 {
		t.Fatal("excluded source read occurred", calls, transport.calls)
	}
	if connected.State().GenerationID != before {
		t.Fatal("rejected acquisition changed the accepted generation")
	}
}

func TestManualAcquisitionUsesConfiguredGitPinAndExplicitOverride(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	connected, err := runtime.Open(t.Context(), runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false), runtime.WithAcquisitionSources(sources.ModelsDevGitID), runtime.WithModelsDevGitCommit(strings.Repeat("a", 40)), runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	}()
	syncer, err := NewForRuntime(connected)
	if err != nil {
		t.Fatal(err)
	}
	before := connected.State().GenerationID
	for _, explicit := range []string{"", "invalid"} {
		opts := []pkgsync.Option{pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "checkout")), pkgsync.WithSkipDepPrompts(true), pkgsync.WithRequireAllSources(true)}
		if explicit != "" {
			opts = append(opts, pkgsync.WithModelsDevGitCommit(explicit))
		}
		_, err := syncer.Sync(t.Context(), opts...)
		if explicit == "" {
			var dependency *pkgerrors.DependencyError
			if !stderrors.As(err, &dependency) {
				t.Fatalf("configured pin did not reach dependency validation: %v", err)
			}
		} else {
			var validation *pkgerrors.ValidationError
			if !stderrors.As(err, &validation) || validation.Field != "ModelsDevGitCommit" || validation.Value != explicit {
				t.Fatalf("explicit manual pin did not replace configured pin: %v", err)
			}
		}
	}
	if connected.State().GenerationID != before {
		t.Fatal("failed acquisition changed the accepted generation")
	}
}
