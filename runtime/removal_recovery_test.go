package runtime

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOperatorRemovalPublicationFailureRecovery(t *testing.T) {
	for _, test := range []struct {
		name              string
		accepted, restore bool
	}{
		{"rejected-remove", false, false}, {"rejected-restore", false, true},
		{"lost-remove-reply", true, false}, {"lost-restore-reply", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var store storage.Store
			var inject, clear func()
			if test.accepted {
				memory := &retentionAmbiguousStore{Memory: storage.NewMemory()}
				store, inject, clear = memory, func() { memory.ambiguous.Store(true) }, func() { memory.ambiguous.Store(false) }
			} else {
				memory := &retentionRejectingStore{Memory: storage.NewMemory()}
				store, inject, clear = memory, func() { memory.reject.Store(true) }, func() { memory.reject.Store(false) }
			}
			connected, options, _ := providerResetRuntime(t, store)
			offering, err := connected.Catalog().Offering("provider", "model")
			if err != nil {
				t.Fatal(err)
			}
			target, err := catalogs.NewCanonicalRemovalTarget(offering.DefinitionID)
			if err != nil {
				t.Fatal(err)
			}
			targets := []catalogs.CatalogRemovalTarget{target}
			if test.restore {
				if _, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), target); err != nil {
					t.Fatal(err)
				}
				targets = nil
			}
			before := connected.State()
			inject()
			if _, err := connected.ReplaceRemovalTargets(t.Context(), before, targets...); err == nil {
				t.Fatal("commit fault returned success")
			}
			if connected.State().GenerationID != before.GenerationID {
				t.Fatal("failed commit reply changed served state")
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			clear()
			restarted := openTestRuntime(t, options...)
			wantRemoved := test.restore
			if test.accepted {
				wantRemoved = !test.restore
			}
			if got := restarted.Catalog().Removals().ContainsCanonical(offering.DefinitionID); got != wantRemoved {
				t.Fatalf("recovered removal = %t, want %t", got, wantRemoved)
			}
			if (restarted.State().GenerationID != before.GenerationID) != test.accepted {
				t.Fatal("recovery selected the wrong commit outcome")
			}
			if pending, err := restarted.store.loadInputPublication(); err != nil || pending != nil {
				t.Fatalf("recovery did not resolve the journal: %v", err)
			}
		})
	}
}

func TestOperatorRemovalConcurrentEditsHaveOneAcceptedWriter(t *testing.T) {
	connected, _, _ := providerResetRuntime(t, storage.NewMemory())
	offering, err := connected.Catalog().Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	target, err := catalogs.NewCanonicalRemovalTarget(offering.DefinitionID)
	if err != nil {
		t.Fatal(err)
	}
	expected := connected.State()
	const writers = 4
	results := make(chan error, writers)
	start := make(chan struct{})
	var group sync.WaitGroup
	for range writers {
		group.Go(func() {
			<-start
			_, err := connected.ReplaceRemovalTargets(t.Context(), expected, target)
			results <- err
		})
	}
	close(start)
	group.Wait()
	close(results)
	accepted, conflicts := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else if errors.IsConflict(err) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || conflicts != writers-1 {
		t.Fatalf("accepted %d writers with %d conflicts", accepted, conflicts)
	}
}

func TestOperatorRemovalMissingPrivateStateRefusesRestart(t *testing.T) {
	connected, options, _ := providerResetRuntime(t, storage.NewMemory())
	offering, err := connected.Catalog().Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	target, err := catalogs.NewCanonicalRemovalTarget(offering.DefinitionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), target); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(connected.store.root, removalPolicyName)
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(t.Context(), options...)
	if err == nil {
		_ = restarted.Close()
		t.Fatal("missing private state silently restored an accepted removal")
	}
}
