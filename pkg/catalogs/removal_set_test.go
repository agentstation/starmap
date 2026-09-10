package catalogs

import (
	"reflect"
	"sync"
	"testing"
)

func TestRemovalSetKeepsScopedAndCanonicalActionsSeparate(t *testing.T) {
	scope := testMembershipScope()
	scoped, err := NewScopedRemovalTarget(scope, "opaque/model")
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewCatalogRemovalSet(scoped)
	if err != nil {
		t.Fatal(err)
	}
	peer := scope
	peer.AccountID = "another-account"
	rotated := scope
	rotated.BindingID, rotated.BindingRevision = "rotated", "2"
	if !set.ContainsScoped(rotated, "opaque/model") || set.ContainsScoped(peer, "opaque/model") || set.ContainsCanonical("author/model") {
		t.Fatal("scoped removal lost account isolation or became canonical")
	}
	canonical, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	set, err = NewCatalogRemovalSet(scoped, canonical)
	if err != nil {
		t.Fatal(err)
	}
	if !set.ContainsCanonical("author/model") || set.ContainsCanonical("author/other") || set.ContainsScoped(peer, "opaque/model") {
		t.Fatal("canonical and scoped queries mixed their targets")
	}
}

func TestRemovalSetOwnsTargetsAndExportsDeterministicOrder(t *testing.T) {
	scope := testMembershipScope()
	scoped, err := NewScopedRemovalTarget(scope, "opaque/model")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	peer := scope
	peer.AccountID = "another-account"
	peerTarget, err := NewScopedRemovalTarget(peer, "opaque/model")
	if err != nil {
		t.Fatal(err)
	}
	otherModel, err := NewScopedRemovalTarget(scope, "other/model")
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewCatalogRemovalSet(scoped, canonical, peerTarget, otherModel, scoped)
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := NewCatalogRemovalSet(otherModel, peerTarget, canonical, scoped)
	if err != nil {
		t.Fatal(err)
	}
	if got := set.Targets(); len(got) != 4 || !reflect.DeepEqual(got, reversed.Targets()) {
		t.Fatal("target order or duplicates changed the removal set")
	}
	scoped.Scope.AccountID = "changed-input"
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for _, target := range set.Targets() {
				if target.Scope != nil {
					target.Scope.AccountID = "changed-export"
				}
			}
			if !set.ContainsScoped(scope, "opaque/model") {
				t.Error("caller mutation reached the retained set")
			}
		})
	}
	workers.Wait()
	if !reflect.DeepEqual(set.Targets(), reversed.Targets()) {
		t.Fatal("caller mutation changed retained targets")
	}
}

func TestRemovalSetRejectsInvalidTargetsAndKeepsEmptyQueriesSafe(t *testing.T) {
	valid, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if set, err := NewCatalogRemovalSet(valid, CatalogRemovalTarget{}); err == nil || set != nil {
		t.Fatal("invalid target produced a removal set")
	}
	for _, set := range []*CatalogRemovalSet{nil, {}} {
		if len(set.Targets()) != 0 || set.ContainsCanonical("author/model") || set.ContainsScoped(testMembershipScope(), "model") {
			t.Fatal("empty removal set excluded an offering")
		}
	}
}

func TestRemovalSetQueriesAllocateNoMemory(t *testing.T) {
	scope := testMembershipScope()
	scoped, err := NewScopedRemovalTarget(scope, "model")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewCatalogRemovalSet(scoped, canonical)
	if err != nil {
		t.Fatal(err)
	}
	var scopedMatch, canonicalMatch bool
	if got := testing.AllocsPerRun(100, func() {
		scopedMatch = set.ContainsScoped(scope, "model")
		canonicalMatch = set.ContainsCanonical("author/model")
	}); got != 0 {
		t.Fatalf("removal queries allocate %v, want zero", got)
	}
	if !scopedMatch || !canonicalMatch {
		t.Fatal("queries did not exercise matching targets")
	}
}
