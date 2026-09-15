package storage

import "testing"

func TestObjectRetentionCapabilityGap(t *testing.T) {
	backend := NewMemoryObjectBackend()
	store, err := NewObject(backend, "retention-gap")
	if err != nil {
		t.Fatal(err)
	}
	previous := ""
	for _, id := range []string{"baseline", "obsolete", "reader", "rollback", "current"} {
		if err := store.Commit(t.Context(), testGeneration(id, id), previous); err != nil {
			t.Fatal(err)
		}
		previous = id
	}
	for _, id := range []string{"baseline", "obsolete", "reader", "rollback", "current"} {
		if _, err := store.Get(t.Context(), id); err != nil {
			t.Fatal(err)
		}
	}
	_, collect := GenerationCollectorFor(store)
	_, lease := GenerationLeaserFor(store)
	t.Logf("five generations readable; collector=%v; generation_leases=%v", collect, lease)
	if !collect || !lease {
		t.Fatal("object catalog storage cannot yet coordinate collection with protected generation readers")
	}
}
