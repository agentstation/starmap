package storage

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

type refusedCoordinatedDeletion struct {
	*MemoryObjectBackend
	refuse bool
}

func (b *refusedCoordinatedDeletion) Delete(ctx context.Context, key, version string) error {
	if b.refuse {
		return &errors.APIError{Provider: "fixture", Endpoint: "delete", StatusCode: 503}
	}
	return b.MemoryObjectBackend.Delete(ctx, key, version)
}

func TestCoordinatedObjectRetriesDeletionAfterRetirement(t *testing.T) {
	objects := &refusedCoordinatedDeletion{MemoryObjectBackend: NewMemoryObjectBackend(), refuse: true}
	store := coordinatedTestStore(t, objects, NewMemoryObjectBackend(), nil)
	if err := store.Commit(t.Context(), testGeneration("old", "old"), ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), testGeneration("current", "current"), "old"); err != nil {
		t.Fatal(err)
	}
	request := RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20}
	report, err := store.Collect(t.Context(), request)
	if err == nil || len(report.Removed) != 0 {
		t.Fatalf("failed deletion reported success: %+v, %v", report, err)
	}
	if _, err := store.Get(t.Context(), "old"); !errors.IsNotFound(err) {
		t.Fatalf("failed physical deletion restored publication rights: %v", err)
	}
	objects.refuse = false
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	page, err := objects.List(t.Context(), ObjectListRequest{Prefix: "catalog/uploads/", Limit: 10})
	if err != nil || len(page.Objects) != 1 {
		t.Fatalf("retry did not recover retired bytes: %+v, %v", page, err)
	}
}

type refusedReaderRelease struct {
	*MemoryObjectBackend
	refuse bool
}

func (b *refusedReaderRelease) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	if b.refuse {
		return ObjectValue{}, &errors.APIError{Provider: "fixture", Endpoint: "coordination", StatusCode: 503}
	}
	return b.MemoryObjectBackend.Put(ctx, key, data, condition)
}

func TestCoordinatedObjectReaderReleaseRemainsRetryable(t *testing.T) {
	coordination := &refusedReaderRelease{MemoryObjectBackend: NewMemoryObjectBackend()}
	store := coordinatedTestStore(t, NewMemoryObjectBackend(), coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
		t.Fatal(err)
	}
	_, release, err := store.AcquireGeneration(t.Context(), "current")
	if err != nil {
		t.Fatal(err)
	}
	coordination.refuse = true
	if err := release(); err == nil {
		t.Fatal("failed release reported success")
	}
	claims, err := store.ReaderClaims(t.Context())
	if err != nil || len(claims.Claims) != 1 {
		t.Fatalf("failed release lost protection: %+v, %v", claims, err)
	}
	coordination.refuse = false
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	claims, err = store.ReaderClaims(t.Context())
	if err != nil || len(claims.Claims) != 0 {
		t.Fatalf("retried release left a claim: %+v, %v", claims, err)
	}
}
