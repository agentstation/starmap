package catalogs

import (
	"fmt"
	"testing"
)

func canonicalAlias(id, target string, state CanonicalAliasState) CanonicalAlias {
	return CanonicalAlias{ID: ModelDefinitionID(id), TargetID: ModelDefinitionID(target), PublisherID: "catalog-publisher", State: state}
}

func TestCanonicalAliasIndexRetainsRemovedEdgesAndCopiesRecords(t *testing.T) {
	records := []CanonicalAlias{
		canonicalAlias("author/old", "author/intermediate", CanonicalAliasActive),
		canonicalAlias("author/intermediate", "author/current", CanonicalAliasRemoved),
	}
	index, err := NewCanonicalAliasIndex([]ModelDefinitionID{"author/current"}, records...)
	if err != nil {
		t.Fatal(err)
	}
	records[0].TargetID = "author/unrelated"
	returned := index.Records()
	returned[0].State = CanonicalAliasActive
	for _, test := range []struct {
		id    ModelDefinitionID
		state CanonicalAliasState
	}{{"author/old", CanonicalAliasActive}, {"author/intermediate", CanonicalAliasRemoved}} {
		target, state, found := index.Lookup(test.id)
		if !found || target != "author/current" || state != test.state {
			t.Fatalf("%s = %s/%s/%t", test.id, target, state, found)
		}
	}
	if _, _, found := index.Lookup("author/missing"); found {
		t.Fatal("unknown ID was reserved")
	}
	var empty *CanonicalAliasIndex
	if _, _, found := empty.Lookup("author/old"); found || len(empty.Records()) != 0 {
		t.Fatal("nil index contains aliases")
	}
	if allocs := testing.AllocsPerRun(100, func() {
		for _, test := range []struct {
			id    ModelDefinitionID
			state CanonicalAliasState
		}{{"author/old", CanonicalAliasActive}, {"author/intermediate", CanonicalAliasRemoved}} {
			if target, state, found := index.Lookup(test.id); !found || target != "author/current" || state != test.state {
				t.Fatal("allocation probe returned the wrong alias")
			}
		}
		if _, _, found := index.Lookup("author/missing"); found {
			t.Fatal("allocation probe found an unknown alias")
		}
	}); allocs != 0 {
		t.Fatalf("alias lookups allocate %v times", allocs)
	}
}

func TestCanonicalAliasIndexRejectsInvalidInventory(t *testing.T) {
	for name, records := range map[string][]CanonicalAlias{
		"missing publisher": {{ID: "author/old", TargetID: "author/current", State: CanonicalAliasActive}},
		"implicit state":    {canonicalAlias("author/old", "author/current", "")},
		"invalid ID":        {canonicalAlias("old", "author/current", CanonicalAliasActive)},
		"invalid target":    {canonicalAlias("author/old", "current", CanonicalAliasActive)},
		"self cycle":        {canonicalAlias("author/old", "author/old", CanonicalAliasActive)},
		"cycle":             {canonicalAlias("author/old", "author/middle", CanonicalAliasRemoved), canonicalAlias("author/middle", "author/old", CanonicalAliasRemoved)},
		"missing terminal":  {canonicalAlias("author/old", "author/missing", CanonicalAliasActive)},
		"shadow live ID":    {canonicalAlias("author/current", "author/missing", CanonicalAliasRemoved)},
		"ambiguous target":  {canonicalAlias("author/old", "author/current", CanonicalAliasActive), canonicalAlias("author/old", "author/missing", CanonicalAliasRemoved)},
		"cross publisher": {
			canonicalAlias("author/old", "author/middle", CanonicalAliasActive),
			{ID: "author/middle", TargetID: "author/current", PublisherID: "other-publisher", State: CanonicalAliasActive},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCanonicalAliasIndex([]ModelDefinitionID{"author/current"}, records...); err == nil {
				t.Fatal("invalid alias inventory was accepted")
			}
		})
	}
	if _, err := NewCanonicalAliasIndex(nil, canonicalAlias("author/old", "author/missing", CanonicalAliasRemoved)); err != nil {
		t.Fatalf("retired identity without a live target: %v", err)
	}
}

func TestCanonicalAliasSuccessorPreservesIdentityThroughRenameAndRestore(t *testing.T) {
	old := canonicalAlias("author/old", "author/current", CanonicalAliasActive)
	previous, err := NewCanonicalAliasIndex([]ModelDefinitionID{"author/current"}, old)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []CanonicalAliasState{CanonicalAliasRemoved, CanonicalAliasActive} {
		old.State = state
		next, err := NewCanonicalAliasIndex([]ModelDefinitionID{"author/next"}, old, canonicalAlias("author/current", "author/next", CanonicalAliasActive))
		if err != nil {
			t.Fatal(err)
		}
		if err := previous.ValidateSuccessor(next); err != nil {
			t.Fatal(err)
		}
		if terminal, actual, found := next.Lookup(old.ID); !found || terminal != "author/next" || actual != state {
			t.Fatalf("successor = %s/%s/%t", terminal, actual, found)
		}
		previous = next
	}
	for name, records := range map[string][]CanonicalAlias{
		"omitted history":    nil,
		"retargeted history": {canonicalAlias("author/old", "author/next", CanonicalAliasActive), canonicalAlias("author/current", "author/next", CanonicalAliasActive)},
		"changed publisher": {
			{ID: "author/old", TargetID: "author/current", PublisherID: "replacement-publisher", State: CanonicalAliasActive},
			{ID: "author/current", TargetID: "author/next", PublisherID: "replacement-publisher", State: CanonicalAliasActive},
		},
	} {
		t.Run(name, func(t *testing.T) {
			next, err := NewCanonicalAliasIndex([]ModelDefinitionID{"author/next"}, records...)
			if err != nil {
				t.Fatal(err)
			}
			if err := previous.ValidateSuccessor(next); err == nil {
				t.Fatal("successor discarded retired identity protection")
			}
		})
	}
	if err := previous.ValidateSuccessor(nil); err == nil {
		t.Fatal("nil successor discarded history")
	}
}

func TestCanonicalAliasIndexFlattensLongChains(t *testing.T) {
	const count = 10000
	records := make([]CanonicalAlias, count)
	for n := range count {
		records[n] = canonicalAlias(fmt.Sprintf("author/model-%05d", n), fmt.Sprintf("author/model-%05d", n+1), CanonicalAliasActive)
	}
	terminal := ModelDefinitionID(fmt.Sprintf("author/model-%05d", count))
	index, err := NewCanonicalAliasIndex([]ModelDefinitionID{terminal}, records...)
	if err != nil {
		t.Fatal(err)
	}
	if actual, state, found := index.Lookup(records[0].ID); !found || actual != terminal || state != CanonicalAliasActive {
		t.Fatalf("long chain = %s/%s/%t", actual, state, found)
	}
}
