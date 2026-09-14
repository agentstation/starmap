package storage

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestMemoryRetentionProtectsCurrentRequiredAndLeasedGenerations(t *testing.T) {
	store := NewMemory()
	retaining, ok := any(store).(RetainingStore)
	if !ok {
		t.Fatal("memory catalog store cannot collect obsolete generations with read leases")
	}
	ctx := t.Context()
	var generations []catalogs.Generation
	previous := ""
	for _, id := range []string{"baseline", "expired", "leased", "pin", "current"} {
		generation := testGeneration(id, id)
		generation.Manifest.GeneratedAt = generation.Manifest.GeneratedAt.Add(time.Duration(len(generations)) * time.Minute)
		generation.Manifest.Validation.ValidatedAt = generation.Manifest.GeneratedAt
		if err := store.Commit(ctx, generation, previous); err != nil {
			t.Fatal(err)
		}
		generations = append(generations, generation)
		previous = id
	}
	leased, release, err := retaining.AcquireGeneration(ctx, "leased")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = release() })
	leased.Payload[0] = '!'
	request := RetentionRequest{ExpectedGenerationID: "current", RequiredGenerationIDs: []string{"baseline", "pin"}, MaxGenerations: 2, MaxBytes: 1 << 20}
	dry := request
	dry.DryRun = true
	preview, err := retaining.Collect(ctx, dry)
	if err != nil || !reflect.DeepEqual(preview.Candidates, []string{"expired"}) || len(preview.Removed) != 0 || preview.After != preview.Before {
		t.Fatalf("dry collection changed storage or selected protected content: %+v, %v", preview, err)
	}
	report, err := retaining.Collect(ctx, request)
	if err != nil || !report.OverLimit || report.Protected.Generations != 4 || report.After.Generations != 4 || !reflect.DeepEqual(report.Removed, []string{"expired"}) {
		t.Fatalf("collection failed to retain protected generations: %+v, %v", report, err)
	}
	for _, index := range []int{0, 2, 3, 4} {
		got, err := store.Get(ctx, generations[index].Manifest.GenerationID)
		if err != nil || !sameGeneration(got, generations[index]) {
			t.Fatalf("protected generation changed: %v", err)
		}
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	report, err = retaining.Collect(ctx, request)
	if err != nil || !reflect.DeepEqual(report.Removed, []string{"leased"}) || report.After.Generations != 3 {
		t.Fatalf("released generation remains protected: %+v, %v", report, err)
	}
}

func TestMemoryRetentionCountsBytesAndBreaksAgeTiesByID(t *testing.T) {
	store := NewMemory()
	first, second, current := testGeneration("a", "old"), testGeneration("b", "recent"), testGeneration("current", "current")
	for index, generation := range []catalogs.Generation{first, second, current} {
		previous := []string{"", "a", "b"}[index]
		if err := store.Commit(t.Context(), generation, previous); err != nil {
			t.Fatal(err)
		}
	}
	size := func(generation catalogs.Generation) int64 {
		manifest, err := json.Marshal(generation.Manifest)
		if err != nil {
			t.Fatal(err)
		}
		return int64(len(manifest) + len(generation.Payload))
	}
	request := RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 3, MaxBytes: size(second) + size(current)}
	report, err := store.Collect(t.Context(), request)
	if err != nil || !reflect.DeepEqual(report.Removed, []string{"a"}) || report.OverLimit {
		t.Fatalf("byte limit or age ordering failed: %+v, %v", report, err)
	}
	if report.Before.Bytes != size(first)+size(second)+size(current) || report.After.Bytes != request.MaxBytes || report.Projected != report.After {
		t.Fatalf("byte totals omit manifest or payload bytes: %+v", report)
	}
	request.MaxBytes = 1
	report, err = store.Collect(t.Context(), request)
	if err != nil || !reflect.DeepEqual(report.Removed, []string{"b"}) || !report.OverLimit || report.Protected.Bytes != size(current) {
		t.Fatalf("oversized current generation lost protection: %+v, %v", report, err)
	}
	if _, err := store.Get(t.Context(), "a"); !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("collected generation remains readable: %v", err)
	}
}

func TestMemoryRetentionReadLeasesHaveIndependentLifetimes(t *testing.T) {
	store := NewMemory()
	first, current := testGeneration("first", "first"), testGeneration("current", "current")
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	_, releaseFirst, err := store.AcquireGeneration(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = releaseFirst() })
	_, releaseSecond, err := store.AcquireGeneration(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = releaseSecond() })
	cancel()
	if err := store.Commit(t.Context(), current, "first"); err != nil {
		t.Fatal(err)
	}
	request := RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			if err := releaseFirst(); err != nil {
				t.Error(err)
			}
			report, err := store.Collect(t.Context(), request)
			if err != nil || !report.OverLimit || len(report.Removed) != 0 {
				t.Errorf("one release ended another caller's lease: %+v, %v", report, err)
			}
		})
	}
	workers.Wait()
	if err := releaseSecond(); err != nil {
		t.Fatal(err)
	}
	report, err := store.Collect(t.Context(), request)
	if err != nil || !reflect.DeepEqual(report.Removed, []string{"first"}) || report.OverLimit {
		t.Fatalf("last release did not end protection: %+v, %v", report, err)
	}
	if _, release, err := store.AcquireGeneration(t.Context(), "first"); !errors.Is(err, pkgerrors.ErrNotFound) || release != nil {
		t.Fatalf("missing generation acquired a lease: %v", err)
	}
}

func TestMemoryRetentionSerializesWithPublication(t *testing.T) {
	for range 32 {
		store := NewMemory()
		first, next := testGeneration("first", "first"), testGeneration("next", "next")
		if err := store.Commit(t.Context(), first, ""); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Go(func() {
			<-start
			if err := store.Commit(t.Context(), next, "first"); err != nil {
				t.Error(err)
			}
		})
		workers.Go(func() {
			<-start
			report, err := store.Collect(t.Context(), RetentionRequest{ExpectedGenerationID: "first", MaxGenerations: 1, MaxBytes: 1})
			if err != nil && !errors.Is(err, pkgerrors.ErrConflict) {
				t.Error(err)
			}
			if len(report.Removed) != 0 {
				t.Errorf("collection removed a current generation during publication: %+v", report)
			}
		})
		close(start)
		workers.Wait()
		current, err := store.Current(t.Context())
		if err != nil || !sameGeneration(current, next) {
			t.Fatalf("collection corrupted publication: %v", err)
		}
	}
}

func TestMemoryRetentionRefusesStaleMissingAndIncompleteRequests(t *testing.T) {
	store := NewMemory()
	retaining, ok := any(store).(RetainingStore)
	if !ok {
		t.Fatal("memory catalog store cannot enforce retention preconditions")
	}
	first, current := testGeneration("first", "first"), testGeneration("current", "current")
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), current, "first"); err != nil {
		t.Fatal(err)
	}
	base := RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20}
	for _, name := range []string{"stale", "missing-required", "scan-limit", "invalid-count", "invalid-bytes", "canceled"} {
		t.Run(name, func(t *testing.T) {
			request := base
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch name {
			case "stale":
				request.ExpectedGenerationID = "first"
			case "missing-required":
				request.RequiredGenerationIDs = []string{"missing"}
			case "scan-limit":
				request.ScanEntries = 1
			case "invalid-count":
				request.MaxGenerations = 0
			case "invalid-bytes":
				request.MaxBytes = 0
			case "canceled":
				cancel()
			}
			report, err := retaining.Collect(ctx, request)
			if err == nil || len(report.Removed) != 0 {
				t.Fatalf("invalid request changed storage: %+v, %v", report, err)
			}
			if name == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if _, err := store.Get(t.Context(), "first"); err != nil {
				t.Fatalf("refusal removed the prior generation: %v", err)
			}
		})
	}
}
