package starmap_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// Allocation counters cover the process, including asynchronous publication hooks.
// A separate process keeps unrelated callback work outside the getter measurement.
func TestEmbeddedCatalogStateAllocations(t *testing.T) {
	const childKey = "STARMAP_TEST_BASELINE_ALLOCATIONS_CHILD"
	if os.Getenv(childKey) != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, executable, "-test.run=^TestEmbeddedCatalogStateAllocations$", "-test.count=1", "-test.timeout=90s", "-test.v")
		command.Env = append(os.Environ(), childKey+"=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated allocation check: %v\n%s", err, output)
		}
		t.Logf("isolated allocation check:\n%s", output)
		return
	}
	client, err := starmap.NewContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewMemory()
	updated, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "stored-only-provider", Name: "Stored Provider"}); err != nil {
		t.Fatal(err)
	}
	custom, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := updated.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		return starmap.NewCandidate(custom, starmap.CandidateEvidence{})
	}); err != nil {
		t.Fatal(err)
	}
	restarted, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		client *starmap.Client
	}{
		{"fresh", client}, {"updated", updated}, {"restarted", restarted},
	} {
		var last starmap.CatalogState
		allocations := testing.AllocsPerRun(100, func() { last = test.client.EmbeddedCatalogState() })
		if allocations != 0 || last.Catalog == nil {
			t.Fatalf("%s baseline reads allocated %g times", test.name, allocations)
		}
		t.Logf("%s baseline allocations = %g", test.name, allocations)
	}
	var control []byte
	controlAllocations := testing.AllocsPerRun(100, func() { control = make([]byte, 64) })
	if controlAllocations < 1 || len(control) != 64 {
		t.Fatalf("allocation control failed: %g allocations", controlAllocations)
	}
	t.Logf("allocating control = %g", controlAllocations)
}
