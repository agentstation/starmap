package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

func coordinatedTestStore(t *testing.T, objects ObjectCollectionBackend, coordination CoordinationBackend, now func() time.Time) *CoordinatedObject {
	t.Helper()
	store, err := NewCoordinatedObject(objects, coordination, CoordinatedObjectConfig{Prefix: "catalog", CoordinationKey: "catalog-state", OwnerID: t.Name(), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestCoordinatedObjectRetainsAcrossIndependentClients(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	first := coordinatedTestStore(t, objects, coordination, nil)
	second := coordinatedTestStore(t, objects, coordination, nil)
	expected := ""
	for _, id := range []string{"baseline", "old", "current"} {
		if err := first.Commit(t.Context(), testGeneration(id, id), expected); err != nil {
			t.Fatal(err)
		}
		expected = id
	}
	ctx, cancel := context.WithCancel(t.Context())
	held, release, err := second.AcquireGeneration(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	request := RetentionRequest{ExpectedGenerationID: "current", RequiredGenerationIDs: []string{"baseline"}, MaxGenerations: 2, MaxBytes: 1 << 20}
	report, err := first.Collect(t.Context(), request)
	if err != nil || !report.OverLimit || len(report.Removed) != 0 {
		t.Fatalf("leased content was not protected: %+v, %v", report, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	report, err = first.Collect(t.Context(), request)
	if err != nil || len(report.Removed) != 1 || report.Removed[0] != "old" {
		t.Fatalf("released generation was not collected: %+v, %v", report, err)
	}
	if _, err := second.Get(t.Context(), "old"); !errors.IsNotFound(err) {
		t.Fatalf("retired generation remained available: %v", err)
	}
	if err := held.Validate(); err != nil {
		t.Fatalf("collection changed caller-owned bytes: %v", err)
	}
	reopened := coordinatedTestStore(t, objects, coordination, nil)
	current, err := reopened.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != "current" {
		t.Fatalf("reopen lost current: %+v, %v", current.Manifest, err)
	}
}

func TestCoordinatedObjectRefusesLostCoordination(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
		t.Fatal(err)
	}
	state, err := coordination.Get(t.Context(), "catalog-state")
	if err != nil {
		t.Fatal(err)
	}
	if err := coordination.Delete(t.Context(), "catalog-state", state.Version); err != nil {
		t.Fatal(err)
	}
	reopened := coordinatedTestStore(t, objects, coordination, nil)
	if _, err := reopened.Current(t.Context()); !errors.IsValidationError(err) {
		t.Fatalf("lost coordination supplied current: %v", err)
	}
	if err := reopened.Commit(t.Context(), testGeneration("replacement", "replacement"), ""); !errors.IsValidationError(err) {
		t.Fatalf("lost coordination reinitialized the store: %v", err)
	}
	if _, err := coordination.Get(t.Context(), "catalog-state"); !errors.IsNotFound(err) {
		t.Fatalf("refusal wrote replacement metadata: %v", err)
	}
}

type delayedCoordinatedUpload struct {
	*MemoryObjectBackend
	entered chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (b *delayedCoordinatedUpload) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	if strings.Contains(key, "/uploads/") {
		b.once.Do(func() { close(b.entered); <-b.resume })
	}
	return b.MemoryObjectBackend.Put(ctx, key, data, condition)
}

func TestCoordinatedObjectFencesExpiredWriterAndCollectsLateBytes(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	now := time.Now().UTC()
	var clockMu sync.Mutex
	clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
	store := coordinatedTestStore(t, objects, coordination, clock)
	if err := store.Commit(t.Context(), testGeneration("baseline", "baseline"), ""); err != nil {
		t.Fatal(err)
	}
	delayed := &delayedCoordinatedUpload{MemoryObjectBackend: objects, entered: make(chan struct{}), resume: make(chan struct{})}
	writer := coordinatedTestStore(t, delayed, coordination, clock)
	var resumeOnce sync.Once
	unblock := func() { resumeOnce.Do(func() { close(delayed.resume) }) }
	t.Cleanup(unblock)
	result := make(chan error, 1)
	go func() { result <- writer.Commit(t.Context(), testGeneration("late", "late"), "baseline") }()
	<-delayed.entered
	clockMu.Lock()
	now = now.Add(2 * DefaultObjectWriteLifetime)
	clockMu.Unlock()
	request := RetentionRequest{ExpectedGenerationID: "baseline", MaxGenerations: 1, MaxBytes: 1 << 20}
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := <-result; !errors.IsConflict(err) {
		t.Fatalf("retired writer published late bytes: %v", err)
	}
	if _, err := store.Get(t.Context(), "late"); !errors.IsNotFound(err) {
		t.Fatalf("late bytes became visible: %v", err)
	}
	if _, err := objects.Put(t.Context(), "catalog/uploads/operator-notes", []byte("preserve"), ObjectPutCondition{IfAbsent: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	page, err := objects.List(t.Context(), ObjectListRequest{Prefix: "catalog/uploads/", Limit: 10})
	if err != nil || len(page.Objects) != 2 {
		t.Fatalf("late upload was not collected or operator data changed: %+v, %v", page, err)
	}
}

func TestCoordinatedObjectConcurrentPublication(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("baseline", "baseline"), ""); err != nil {
		t.Fatal(err)
	}
	outcomes := make(chan error, 8)
	var workers sync.WaitGroup
	for index := range 8 {
		workers.Go(func() {
			id := fmt.Sprintf("candidate-%d", index)
			other := coordinatedTestStore(t, objects, coordination, nil)
			outcomes <- other.Commit(t.Context(), testGeneration(id, id), "baseline")
		})
	}
	workers.Wait()
	close(outcomes)
	succeeded, conflicted := 0, 0
	for err := range outcomes {
		if err == nil {
			succeeded++
		} else if errors.IsConflict(err) {
			conflicted++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || conflicted != 7 {
		t.Fatalf("publication admitted %d writers and refused %d", succeeded, conflicted)
	}
}

func TestCoordinatedObjectRetentionRefusalsPreserveStorage(t *testing.T) {
	for _, mode := range []string{"stale-current", "missing-required", "scan-limit", "invalid-limit", "dry-run"} {
		t.Run(mode, func(t *testing.T) {
			objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
			store := coordinatedTestStore(t, objects, coordination, nil)
			if err := store.Commit(t.Context(), testGeneration("old", "old"), ""); err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(t.Context(), testGeneration("current", "current"), "old"); err != nil {
				t.Fatal(err)
			}
			before, err := coordination.Get(t.Context(), "catalog-state")
			if err != nil {
				t.Fatal(err)
			}
			request := RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20}
			switch mode {
			case "stale-current":
				request.ExpectedGenerationID = "old"
			case "missing-required":
				request.RequiredGenerationIDs = []string{"absent"}
			case "scan-limit":
				request.ScanEntries = 1
			case "invalid-limit":
				request.MaxBytes = 0
			case "dry-run":
				request.DryRun = true
			}
			report, err := store.Collect(t.Context(), request)
			if mode == "dry-run" {
				if err != nil || len(report.Candidates) != 1 || len(report.Removed) != 0 || report.After != report.Before {
					t.Fatalf("dry run: %+v, %v", report, err)
				}
			} else if err == nil {
				t.Fatal("invalid retention request succeeded")
			}
			after, err := coordination.Get(t.Context(), "catalog-state")
			if err != nil || before.Version != after.Version || !bytes.Equal(before.Data, after.Data) {
				t.Fatalf("refusal changed coordination: %v", err)
			}
			page, err := objects.List(t.Context(), ObjectListRequest{Prefix: "catalog/uploads/", Limit: 10})
			if err != nil || len(page.Objects) != 2 {
				t.Fatalf("refusal changed stored generations: %+v, %v", page, err)
			}
		})
	}
}

func TestCoordinatedObjectReaderRecoveryRequiresExactRevision(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	first := coordinatedTestStore(t, objects, coordination, nil)
	second, err := NewCoordinatedObject(objects, coordination, CoordinatedObjectConfig{Prefix: "catalog", CoordinationKey: "catalog-state", OwnerID: "other-process"})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
		t.Fatal(err)
	}
	_, releaseFirst, err := first.AcquireGeneration(t.Context(), "current")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := first.ReaderClaims(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, releaseSecond, err := second.AcquireGeneration(t.Context(), "current")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.ReleaseFencedOwner(t.Context(), t.Name(), claims.Revision); !errors.IsConflict(err) {
		t.Fatalf("stale recovery removed live claims: %v", err)
	}
	claims, err = first.ReaderClaims(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.ReleaseFencedOwner(t.Context(), t.Name(), claims.Revision); err != nil {
		t.Fatal(err)
	}
	remaining, err := first.ReaderClaims(t.Context())
	if err != nil || len(remaining.Claims) != 1 || remaining.Claims[0].OwnerID != "other-process" {
		t.Fatalf("recovery removed another owner's protection: %+v, %v", remaining, err)
	}
	if err := releaseFirst(); err != nil {
		t.Fatal(err)
	}
	if err := releaseSecond(); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinatedObjectRejectsMalformedRegistry(t *testing.T) {
	for _, mode := range []string{"duplicate", "schema", "upload", "current", "reader", "digest"} {
		t.Run(mode, func(t *testing.T) {
			objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
			store := coordinatedTestStore(t, objects, coordination, nil)
			if err := store.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
				t.Fatal(err)
			}
			original, err := coordination.Get(t.Context(), "catalog-state")
			if err != nil {
				t.Fatal(err)
			}
			var state coordinatedRegistry
			if err := json.Unmarshal(original.Data, &state); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "digest":
				entry := state.Generations["current"]
				entry.ManifestDigest = strings.Repeat("z", 64)
				state.Generations["current"] = entry
			case "schema":
				state.Version++
			case "upload":
				entry := state.Generations["current"]
				entry.UploadID = "../../operator"
				state.Generations["current"] = entry
			case "current":
				state.Current = "missing"
			case "reader":
				state.Readers["invalid"] = coordinatedReader{OwnerID: "operator", UploadID: "absent"}
			}
			damaged, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "duplicate" {
				damaged = bytes.Replace(damaged, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
			}
			stored, err := coordination.Put(t.Context(), "catalog-state", damaged, ObjectPutCondition{IfVersion: original.Version})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Current(t.Context()); !errors.IsValidationError(err) {
				t.Fatalf("invalid registry supplied current: %v", err)
			}
			if _, err := store.Collect(t.Context(), RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1}); !errors.IsValidationError(err) {
				t.Fatalf("invalid registry allowed collection: %v", err)
			}
			after, err := coordination.Get(t.Context(), "catalog-state")
			if err != nil || after.Version != stored.Version || !bytes.Equal(after.Data, stored.Data) {
				t.Fatalf("refusal changed damaged metadata: %v", err)
			}
		})
	}
}

func TestCoordinatedObjectConstructionAndColdReadsArePassive(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	if _, err := store.Current(t.Context()); !errors.IsNotFound(err) {
		t.Fatalf("empty storage supplied current: %v", err)
	}
	if len(objects.objects) != 0 || len(coordination.objects) != 0 {
		t.Fatal("construction or a cold read initialized storage")
	}
	if err := store.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
		t.Fatal(err)
	}
	plain, err := NewObject(objects, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	if err := plain.Commit(t.Context(), testGeneration("plain", "plain"), ""); !errors.IsValidationError(err) {
		t.Fatalf("plain writer bypassed coordinated mode: %v", err)
	}
}

func TestCoordinatedObjectRecoveryByteBoundPreservesStorage(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
		t.Fatal(err)
	}
	state, version, err := store.load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	generation := testGeneration("orphan", "orphan")
	entry, manifest, err := coordinatedCandidate(t.Context(), generation)
	if err != nil {
		t.Fatal(err)
	}
	entry.UploadID = rand.Text()
	data, err := encodeCoordinatedObject(state, entry, manifest, generation.Payload)
	if err != nil {
		t.Fatal(err)
	}
	key := store.uploadKey(entry.UploadID)
	if _, err := objects.Put(t.Context(), key, data, ObjectPutCondition{IfAbsent: true}); err != nil {
		t.Fatal(err)
	}
	request := RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20, InputMaxBytes: 1}
	if _, err := store.Collect(t.Context(), request); !errors.IsValidationError(err) {
		t.Fatalf("recovery ignored the byte bound: %v", err)
	}
	after, err := coordination.Get(t.Context(), "catalog-state")
	if err != nil || after.Version != version {
		t.Fatalf("bounded refusal changed coordination: %v", err)
	}
	if _, err := objects.Get(t.Context(), key); err != nil {
		t.Fatalf("bounded refusal removed the orphan: %v", err)
	}
	request.InputMaxBytes = int64(len(data))
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := objects.Get(t.Context(), key); !errors.IsNotFound(err) {
		t.Fatalf("qualified recovery retained orphan bytes: %v", err)
	}
}

type refuseCoordinatedPayloadReads struct{ *MemoryObjectBackend }

func (b refuseCoordinatedPayloadReads) Get(ctx context.Context, key string) (ObjectValue, error) {
	if strings.Contains(key, "/uploads/") {
		return ObjectValue{}, &errors.ValidationError{Field: "test", Message: "payload reads are disabled"}
	}
	return b.MemoryObjectBackend.Get(ctx, key)
}

func TestCoordinatedObjectCollectionUsesCommittedVersions(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("old", "old"), ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), testGeneration("current", "current"), "old"); err != nil {
		t.Fatal(err)
	}
	collector := coordinatedTestStore(t, refuseCoordinatedPayloadReads{objects}, coordination, nil)
	report, err := collector.Collect(t.Context(), RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20})
	if err != nil || len(report.Removed) != 1 || report.Removed[0] != "old" {
		t.Fatalf("collection required catalog downloads: %+v, %v", report, err)
	}
}

func TestCoordinatedObjectAuthorityObservationUsesCurrentMetadata(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	generation := authorityStoredGeneration("authority", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	reader := coordinatedTestStore(t, refuseCoordinatedPayloadReads{objects}, coordination, nil)
	before, err := coordination.Get(t.Context(), "catalog-state")
	if err != nil {
		t.Fatal(err)
	}
	head, err := reader.CurrentAuthorityHead(t.Context())
	if err != nil || head != generation.Manifest.AuthorityHead {
		t.Fatalf("authority required a catalog payload read: %+v, %v", head, err)
	}
	after, err := coordination.Get(t.Context(), "catalog-state")
	if err != nil || before.Version != after.Version || !bytes.Equal(before.Data, after.Data) {
		t.Fatalf("authority observation changed metadata: %v", err)
	}
}

type ambiguousCoordinatedPublication struct {
	*MemoryObjectBackend
	lost bool
}

func (b *ambiguousCoordinatedPublication) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	value, err := b.MemoryObjectBackend.Put(ctx, key, data, condition)
	if err == nil && !b.lost && bytes.Contains(data, []byte(`"current":"candidate"`)) {
		b.lost = true
		return ObjectValue{}, &errors.ValidationError{Field: "test", Message: "publication response was lost"}
	}
	return value, err
}

func TestCoordinatedObjectRetriesAmbiguousPublication(t *testing.T) {
	objects := NewMemoryObjectBackend()
	coordination := &ambiguousCoordinatedPublication{MemoryObjectBackend: NewMemoryObjectBackend()}
	store := coordinatedTestStore(t, objects, coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("baseline", "baseline"), ""); err != nil {
		t.Fatal(err)
	}
	candidate := testGeneration("candidate", "candidate")
	if err := store.Commit(t.Context(), candidate, "baseline"); err == nil {
		t.Fatal("fixture did not lose the publication response")
	}
	current, err := store.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != "candidate" {
		t.Fatalf("ambiguous publication lost current: %+v, %v", current.Manifest, err)
	}
	if err := store.Commit(t.Context(), candidate, "baseline"); err != nil {
		t.Fatalf("identical retry failed: %v", err)
	}
}

type delayedCoordinatedDeletion struct {
	*MemoryObjectBackend
	entered chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (b *delayedCoordinatedDeletion) Delete(ctx context.Context, key, version string) error {
	b.once.Do(func() { close(b.entered); <-b.resume })
	return b.MemoryObjectBackend.Delete(ctx, key, version)
}

func TestCoordinatedObjectLateDeletionCannotRemoveRecreatedGeneration(t *testing.T) {
	objects, coordination := NewMemoryObjectBackend(), NewMemoryObjectBackend()
	store := coordinatedTestStore(t, objects, coordination, nil)
	original := testGeneration("old", "identical-content")
	if err := store.Commit(t.Context(), original, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), testGeneration("current", "current"), "old"); err != nil {
		t.Fatal(err)
	}
	delayed := &delayedCoordinatedDeletion{MemoryObjectBackend: objects, entered: make(chan struct{}), resume: make(chan struct{})}
	collector := coordinatedTestStore(t, delayed, coordination, nil)
	var resumeOnce sync.Once
	unblock := func() { resumeOnce.Do(func() { close(delayed.resume) }) }
	t.Cleanup(unblock)
	result := make(chan error, 1)
	go func() {
		_, err := collector.Collect(t.Context(), RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20})
		result <- err
	}()
	<-delayed.entered
	if err := store.Commit(t.Context(), original, "current"); err != nil {
		t.Fatalf("new upload reused an object pending deletion: %v", err)
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	current, err := store.Current(t.Context())
	if err != nil || !sameGeneration(current, original) {
		t.Fatalf("old deletion removed the new accepted generation: %+v, %v", current.Manifest, err)
	}
}

type interruptedReaderClaim struct {
	*MemoryObjectBackend
	lost     bool
	failRead bool
}

func (b *interruptedReaderClaim) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	value, err := b.MemoryObjectBackend.Put(ctx, key, data, condition)
	var state coordinatedRegistry
	if err == nil && !b.lost && json.Unmarshal(data, &state) == nil && len(state.Readers) > 0 {
		b.lost, b.failRead = true, true
		return ObjectValue{}, &errors.ConflictError{Resource: "test coordination response", Actual: value.Version}
	}
	return value, err
}

func (b *interruptedReaderClaim) GetCurrent(ctx context.Context, key string) (ObjectValue, error) {
	if b.failRead {
		b.failRead = false
		return ObjectValue{}, &errors.APIError{Provider: "fixture", Endpoint: "coordination", StatusCode: 503}
	}
	return b.MemoryObjectBackend.GetCurrent(ctx, key)
}

func TestCoordinatedObjectFailedAcquisitionReleasesAnAmbiguousClaim(t *testing.T) {
	objects := NewMemoryObjectBackend()
	coordination := &interruptedReaderClaim{MemoryObjectBackend: NewMemoryObjectBackend()}
	store := coordinatedTestStore(t, objects, coordination, nil)
	if err := store.Commit(t.Context(), testGeneration("current", "current"), ""); err != nil {
		t.Fatal(err)
	}
	if _, release, err := store.AcquireGeneration(t.Context(), "current"); err == nil || release != nil {
		t.Fatal("fixture did not interrupt acquisition")
	}
	claims, err := store.ReaderClaims(t.Context())
	if err != nil || len(claims.Claims) != 0 {
		t.Fatalf("failed acquisition left a permanent protection claim: %+v, %v", claims, err)
	}
}
