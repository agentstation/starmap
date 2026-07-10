package catalogstore

import (
	"context"
	stderrors "errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type faultObjectBackend struct {
	delegate ObjectBackend
	failPut  func(string) error
}

func (b *faultObjectBackend) Get(ctx context.Context, key string) (ObjectValue, error) {
	return b.delegate.Get(ctx, key)
}

func (b *faultObjectBackend) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	if b.failPut != nil {
		if err := b.failPut(key); err != nil {
			return ObjectValue{}, err
		}
	}
	return b.delegate.Put(ctx, key, data, condition)
}

func TestObjectCatalogStoreUploadPromotionRollbackFaults(t *testing.T) {
	backend := &faultObjectBackend{delegate: NewMemoryObjectBackend()}
	store, err := NewObject(backend, "faults")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	first := testGeneration("object-fault-first", "first")
	if err := store.Commit(context.Background(), first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}

	uploadFault := stderrors.New("injected payload upload failure")
	backend.failPut = func(key string) error {
		if strings.HasSuffix(key, "/"+payloadFilename) {
			return uploadFault
		}
		return nil
	}
	second := testGeneration("object-fault-second", "second")
	if err := store.Commit(context.Background(), second, "object-fault-first"); !stderrors.Is(err, uploadFault) {
		t.Fatalf("payload-fault Commit error = %v", err)
	}
	assertStoredGeneration(t, store, first)
	if _, err := store.Get(context.Background(), "object-fault-second"); err == nil {
		t.Fatal("incomplete uploaded candidate was readable")
	}

	backend.failPut = nil
	if err := store.Commit(context.Background(), second, "object-fault-first"); err != nil {
		t.Fatalf("retry second: %v", err)
	}
	assertStoredGeneration(t, store, second)

	promotionFault := stderrors.New("injected pointer promotion failure")
	backend.failPut = func(key string) error {
		if key == store.currentKey() {
			return promotionFault
		}
		return nil
	}
	third := testGeneration("object-fault-third", "third")
	if err := store.Commit(context.Background(), third, "object-fault-second"); !stderrors.Is(err, promotionFault) {
		t.Fatalf("promotion-fault Commit error = %v", err)
	}
	assertStoredGeneration(t, store, second)
	staged, err := store.Get(context.Background(), "object-fault-third")
	if err != nil {
		t.Fatalf("complete staged third: %v", err)
	}
	if diff := cmp.Diff(third, staged); diff != "" {
		t.Fatalf("staged third mismatch (-want +got):\n%s", diff)
	}

	backend.failPut = nil
	if err := store.Commit(context.Background(), third, "object-fault-second"); err != nil {
		t.Fatalf("retry third: %v", err)
	}
	if err := store.Commit(context.Background(), first, "object-fault-third"); err != nil {
		t.Fatalf("rollback to retained first: %v", err)
	}
	assertStoredGeneration(t, store, first)
}

func TestObjectCatalogStoreRejectsCorruptStoredPayload(t *testing.T) {
	backend := NewMemoryObjectBackend()
	store, err := NewObject(backend, "corrupt")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	generation := testGeneration("object-corrupt", "original")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	key := store.generationKey(generation.Manifest.GenerationID, payloadFilename)
	object, err := backend.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get payload object: %v", err)
	}
	if _, err := backend.Put(
		context.Background(), key, []byte("corrupt"), ObjectPutCondition{IfVersion: object.Version},
	); err != nil {
		t.Fatalf("corrupt payload object: %v", err)
	}
	_, err = store.Get(context.Background(), generation.Manifest.GenerationID)
	if !pkgerrors.IsValidationError(err) {
		t.Fatalf("Get corrupt payload error = %v, want validation error", err)
	}
}

type recordingObjectBackend struct {
	delegate ObjectBackend
	gets     atomic.Int64
	puts     atomic.Int64
}

func (b *recordingObjectBackend) Get(ctx context.Context, key string) (ObjectValue, error) {
	b.gets.Add(1)
	return b.delegate.Get(ctx, key)
}

func (b *recordingObjectBackend) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	b.puts.Add(1)
	return b.delegate.Put(ctx, key, data, condition)
}

func TestSeamConformanceObjectStoreAcceptsAlternateBackend(t *testing.T) {
	backend := &recordingObjectBackend{delegate: NewMemoryObjectBackend()}
	store, err := NewObject(backend, "alternate")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	want := testGeneration("object-generation", "object")
	if err := store.Commit(context.Background(), want, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	got, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("generation mismatch (-want +got):\n%s", diff)
	}
	if backend.gets.Load() == 0 || backend.puts.Load() == 0 {
		t.Fatalf("alternate backend calls: gets=%d puts=%d", backend.gets.Load(), backend.puts.Load())
	}
}

func TestObjectCatalogStoreReopensCurrentGeneration(t *testing.T) {
	backend := NewMemoryObjectBackend()
	first, err := NewObject(backend, "reopen")
	if err != nil {
		t.Fatalf("NewObject first: %v", err)
	}
	want := testGeneration("object-reopen", "durable")
	if err := first.Commit(context.Background(), want, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	reopened, err := NewObject(backend, "reopen")
	if err != nil {
		t.Fatalf("NewObject reopened: %v", err)
	}
	got, err := reopened.Current(context.Background())
	if err != nil {
		t.Fatalf("Current reopened: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("reopened generation mismatch (-want +got):\n%s", diff)
	}
}

var _ ObjectBackend = (*recordingObjectBackend)(nil)
var _ ObjectBackend = (*faultObjectBackend)(nil)
