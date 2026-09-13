package storage

import (
	"context"
	stderrors "errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestMemoryObjectCollectionPages(t *testing.T) {
	backend := NewMemoryObjectBackend()
	for _, key := range []string{"catalog/a", "catalog/b", "catalog/c", "other/a"} {
		if _, err := backend.Put(t.Context(), key, []byte(key), ObjectPutCondition{IfAbsent: true}); err != nil {
			t.Fatal(err)
		}
	}
	collection, ok := any(backend).(ObjectCollectionBackend)
	if !ok {
		t.Fatal("memory object backend cannot enumerate retained catalog objects")
	}
	page, err := collection.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Limit: 2})
	if err != nil || len(page.Objects) != 2 || page.Next == "" {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	if page.Objects[0].Key != "catalog/a" || page.Objects[1].Key != "catalog/b" {
		t.Fatalf("first page keys = %+v", page.Objects)
	}
	for _, entry := range page.Objects {
		value, err := backend.Get(t.Context(), entry.Key)
		if err != nil || entry.Version != value.Version || entry.Size != int64(len(value.Data)) {
			t.Fatalf("entry = %+v, value = %+v, error = %v", entry, value, err)
		}
	}
	page, err = collection.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Cursor: page.Next, Limit: 2})
	if err != nil || len(page.Objects) != 1 || page.Objects[0].Key != "catalog/c" || page.Next != "" {
		t.Fatalf("last page = %+v, %v", page, err)
	}
}

func TestMemoryObjectCollectionPageBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3, 17, 1001} {
		for _, limit := range []int{1, 2, 16, 1000} {
			t.Run(fmt.Sprintf("%d-%d", count, limit), func(t *testing.T) {
				backend := NewMemoryObjectBackend()
				var want []string
				for i := range count {
					key := fmt.Sprintf("catalog/%04d", i)
					want = append(want, key)
					if _, err := backend.Put(t.Context(), key, []byte(key), ObjectPutCondition{IfAbsent: true}); err != nil {
						t.Fatal(err)
					}
				}
				var got []string
				cursor := ""
				for pages := 0; ; pages++ {
					if pages > count {
						t.Fatal("inventory cursor did not terminate")
					}
					page, err := backend.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Cursor: cursor, Limit: limit})
					if err != nil || len(page.Objects) > limit {
						t.Fatalf("page = %+v, %v", page, err)
					}
					for _, entry := range page.Objects {
						got = append(got, entry.Key)
					}
					if page.Next == "" {
						break
					}
					if page.Next == cursor {
						t.Fatal("cursor did not advance")
					}
					cursor = page.Next
				}
				if !slices.Equal(got, want) {
					t.Fatalf("inventory omitted or repeated objects: got %d, want %d", len(got), len(want))
				}
			})
		}
	}
}

func TestMemoryObjectCollectionRejectsInvalidOperations(t *testing.T) {
	backend := NewMemoryObjectBackend()
	value, err := backend.Put(t.Context(), "catalog/a", []byte("keep"), ObjectPutCondition{IfAbsent: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []ObjectListRequest{{Limit: 1}, {Prefix: "catalog/"}, {Prefix: "catalog/", Limit: -1}, {Prefix: "catalog/", Limit: MaxObjectListEntries + 1}, {Prefix: "other/", Limit: 1, Cursor: "catalog/a"}} {
		if _, err := backend.List(t.Context(), request); err == nil {
			t.Fatalf("invalid request accepted: %+v", request)
		}
	}
	for _, version := range []string{"", " ", "*"} {
		if err := backend.Delete(t.Context(), "catalog/a", version); err == nil {
			t.Fatalf("invalid validator accepted: %q", version)
		}
	}
	if err := backend.Delete(t.Context(), "", value.Version); err == nil {
		t.Fatal("empty key accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := backend.List(ctx, ObjectListRequest{Prefix: "catalog/", Limit: 1}); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled list = %v", err)
	}
	if err := backend.Delete(ctx, "catalog/a", value.Version); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled delete = %v", err)
	}
	got, err := backend.Get(t.Context(), "catalog/a")
	if err != nil || string(got.Data) != "keep" {
		t.Fatalf("object changed: %+v, %v", got, err)
	}
	page, err := backend.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	page.Objects[0].Key = "changed"
	page.Objects[0].Version = "changed"
	page.Objects[0].Size = -1
	again, err := backend.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Limit: 1})
	if err != nil || again.Objects[0] != (ObjectEntry{Key: "catalog/a", Version: value.Version, Size: 4}) {
		t.Fatalf("caller mutated stored entry: %+v, %v", again, err)
	}
	if err := backend.Delete(t.Context(), "missing", value.Version); !errors.IsNotFound(err) {
		t.Fatalf("missing deletion = %v", err)
	}
}

func TestMemoryObjectCollectionConcurrentReplacement(t *testing.T) {
	backend := NewMemoryObjectBackend()
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for i := range 32 {
				key := fmt.Sprintf("catalog/%d-%d", worker, i)
				old, err := backend.Put(t.Context(), key, []byte("old"), ObjectPutCondition{IfAbsent: true})
				if err != nil {
					t.Error(err)
					return
				}
				current, err := backend.Put(t.Context(), key, []byte("new"), ObjectPutCondition{IfVersion: old.Version})
				if err != nil {
					t.Error(err)
					return
				}
				if err := backend.Delete(t.Context(), key, old.Version); !isConflict(err) {
					t.Errorf("stale delete = %v", err)
					return
				}
				if _, err := backend.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Limit: 3}); err != nil {
					t.Error(err)
					return
				}
				if err := backend.Delete(t.Context(), key, current.Version); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	workers.Wait()
	page, err := backend.List(t.Context(), ObjectListRequest{Prefix: "catalog/", Limit: 1})
	if err != nil || len(page.Objects) != 0 || page.Next != "" {
		t.Fatalf("final inventory = %+v, %v", page, err)
	}
}

func TestMemoryObjectCollectionConditionalDelete(t *testing.T) {
	backend := NewMemoryObjectBackend()
	original, err := backend.Put(t.Context(), "catalog/a", []byte("old"), ObjectPutCondition{IfAbsent: true})
	if err != nil {
		t.Fatal(err)
	}
	current, err := backend.Put(t.Context(), "catalog/a", []byte("new"), ObjectPutCondition{IfVersion: original.Version})
	if err != nil {
		t.Fatal(err)
	}
	collection, ok := any(backend).(ObjectCollectionBackend)
	if !ok {
		t.Fatal("memory object backend cannot conditionally delete a retained object")
	}
	if err := collection.Delete(t.Context(), "catalog/a", original.Version); !isConflict(err) {
		t.Fatalf("stale delete = %v", err)
	}
	value, err := backend.Get(t.Context(), "catalog/a")
	if err != nil || string(value.Data) != "new" {
		t.Fatalf("replacement = %+v, %v", value, err)
	}
	if err := collection.Delete(t.Context(), "catalog/a", current.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Get(t.Context(), "catalog/a"); !errors.IsNotFound(err) {
		t.Fatalf("deleted object = %v", err)
	}
}
