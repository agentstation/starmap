package acquisition

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestOfflineManualAcquisitionDoesNotResolveCredentials(t *testing.T) {
	connected, err := runtime.Open(t.Context(), runtime.WithCatalogSource("embedded"),
		runtime.WithCatalogNetworkMode("offline"), runtime.WithStateDirectory(filepath.Join(t.TempDir(), "runtime")),
		runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connected.Close() })
	var resolutions atomic.Int64
	syncer, err := NewForRuntime(connected, WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		resolutions.Add(1)
		return sources.ProviderCredentialMaterial{}, &errors.ValidationError{Field: "test credential", Message: "unavailable"}
	})))
	if err != nil {
		t.Fatal(err)
	}
	before := connected.State().GenerationID
	for _, dryRun := range []bool{false, true} {
		_, err := syncer.Sync(t.Context(), pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider("openai"), pkgsync.WithDryRun(dryRun))
		if err == nil {
			t.Error("offline manual acquisition returned no refusal")
		}
	}
	if got := resolutions.Load(); got != 0 {
		t.Fatalf("offline manual acquisition resolved %d credentials", got)
	}
	if connected.State().GenerationID != before {
		t.Fatal("offline acquisition changed the accepted generation")
	}
}
