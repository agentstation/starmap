# Budget storage probe

This test uses a Go overlay. Product source remains unchanged.

```go
package storage

import (
	"errors"
	"os"
	"testing"
)

func TestBudgetTransactionCASContract(t *testing.T) {
	for _, backend := range []string{"badger", "valkey"} {
		t.Run(backend, func(t *testing.T) {
			for _, phase := range []string{"wrong_expected", "change_before_commit"} {
				t.Run(phase, func(t *testing.T) {
					var store KVStore
					var err error
					if backend == "badger" {
						store, err = OpenBadger(BadgerConfig{Path: t.TempDir(), SyncWrites: true, Compression: true, NumVersions: 1, NumLevelZero: 5, MemTableSize: 64 << 20})
					} else {
						address := os.Getenv("TEST_VALKEY_URL")
						if address == "" {
							t.Fatal("real Valkey address required")
						}
						store, err = OpenValkey(ValkeyConfig{URL: address})
					}
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = store.Close() })
					ctx := t.Context()
					key := contractKey(t, "budget")
					if err := store.Set(ctx, key, []byte("100")); err != nil {
						t.Fatal(err)
					}
					tx, err := store.BeginTransaction(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback()
					expected := "wrong"
					want := "100"
					if phase == "change_before_commit" {
						expected = "100"
						want = "80"
					}
					err = tx.CompareAndSwap(key, []byte(expected), []byte("40"))
					if err == nil {
						if phase == "change_before_commit" {
							if err := store.Set(ctx, key, []byte(want)); err != nil {
								t.Fatal(err)
							}
						}
						err = tx.Commit(ctx)
					}
					if !errors.Is(err, ErrConflict) {
						t.Errorf("transaction conflict = %v; want ErrConflict", err)
					}
					got, err := store.Get(ctx, key)
					if err != nil {
						t.Fatal(err)
					}
					if string(got) != want {
						t.Errorf("stored capacity = %s; want unchanged %s", got, want)
					}
				})
			}
		})
	}
}

func TestBudgetBatchCASContract(t *testing.T) {
	for _, backend := range []string{"badger", "valkey"} {
		t.Run(backend, func(t *testing.T) {
			var store KVStore
			var err error
			if backend == "badger" {
				store, err = OpenBadger(BadgerConfig{Path: t.TempDir(), SyncWrites: true, Compression: true, NumVersions: 1, NumLevelZero: 5, MemTableSize: 64 << 20})
			} else {
				address := os.Getenv("TEST_VALKEY_URL")
				if address == "" {
					t.Fatal("real Valkey address required")
				}
				store, err = OpenValkey(ValkeyConfig{URL: address})
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			ctx := t.Context()
			mutations := make([]CompareAndSwapMutation, 0, 3)
			for _, scope := range []string{"account", "key", "team"} {
				key := contractKey(t, scope)
				if err := store.Set(ctx, key, []byte("1000")); err != nil {
					t.Fatal(err)
				}
				mutations = append(mutations, CompareAndSwapMutation{Key: key, ExpectedValue: []byte("1000"), NewValue: []byte("400")})
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			for range 2 {
				go func() { <-start; results <- store.CompareAndSwapBatch(ctx, mutations) }()
			}
			close(start)
			successes, conflicts := 0, 0
			for range 2 {
				err := <-results
				if err == nil {
					successes++
				} else if errors.Is(err, ErrConflict) {
					conflicts++
				} else {
					t.Errorf("batch: %v", err)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("successes=%d conflicts=%d; want one each", successes, conflicts)
			}
			for _, m := range mutations {
				got, err := store.Get(ctx, m.Key)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != "400" {
					t.Errorf("capacity=%s; want 400", got)
				}
			}
			for i := range mutations {
				mutations[i].ExpectedValue = []byte("400")
				mutations[i].NewValue = []byte("200")
			}
			mutations[2].ExpectedValue = []byte("stale-team")
			if err := store.CompareAndSwapBatch(ctx, mutations); !errors.Is(err, ErrConflict) {
				t.Errorf("mixed batch=%v; want conflict", err)
			}
			for _, m := range mutations {
				got, err := store.Get(ctx, m.Key)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != "400" {
					t.Errorf("partial reservation=%s; want unchanged 400", got)
				}
			}
		})
	}
}
```
