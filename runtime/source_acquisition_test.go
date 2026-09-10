package runtime

import (
	"context"
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

type sourceAcquirerFunc func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error)

func (f sourceAcquirerFunc) AcquireSources(ctx context.Context, request SourceAcquisitionRequest) ([]sources.Observation, error) {
	return f(ctx, request)
}

func acquisitionSourceObservation(t *testing.T, partial bool) sources.Observation {
	t.Helper()
	catalog, err := catalogs.DecodeCatalogPayload(testCatalogPayload(t, "metadata", "model", "Metadata"))
	if err != nil {
		t.Fatal(err)
	}
	metadata := sources.ObservationMetadata{ObservedAt: time.Now().UTC(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Records: sources.ObservationRecordCounts{Accepted: 1}}
	if partial {
		metadata.Status, metadata.Completeness = sources.ObservationStatusDegraded, sources.ObservationCompletenessPartial
		metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "metadata/invalid", Message: "invalid fixture record"}}
	}
	observation, err := sources.NewObservation(sources.LocalCatalogID, catalog, metadata)
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestSourceAndProviderAcquisitionPublishIndependently(t *testing.T) {
	for _, slow := range []string{"provider", "metadata"} {
		t.Run(slow, func(t *testing.T) {
			observation := acquisitionSourceObservation(t, false)
			layer := testProviderLayer(t, "provider", "model", "Provider", time.Now().UTC())
			release := make(chan struct{})
			var unblock sync.Once
			defer unblock.Do(func() { close(release) })
			wait := func(ctx context.Context) error {
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			provider := windowAcquirerFunc(func(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
				if slow == "provider" {
					if err := wait(ctx); err != nil {
						return AcquisitionResult{}, err
					}
				}
				if err := request.Publish(ctx, []ProviderLayer{layer}); err != nil {
					return AcquisitionResult{}, err
				}
				return AcquisitionResult{Layers: []ProviderLayer{layer}}, nil
			})
			metadata := sourceAcquirerFunc(func(ctx context.Context, request SourceAcquisitionRequest) ([]sources.Observation, error) {
				if request.Current == nil {
					t.Error("source request has no current catalog")
				}
				if slow == "metadata" {
					if err := wait(ctx); err != nil {
						return nil, err
					}
				}
				return []sources.Observation{observation}, nil
			})
			connected := openTestRuntime(t, WithCatalogSource("embedded"), WithAcquirer(provider), WithSourceAcquirer(metadata), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
			result := make(chan error, 1)
			go func() { _, err := connected.Sync(t.Context()); result <- err }()
			fast := catalogs.ProviderID("metadata")
			if slow == "metadata" {
				fast = "provider"
			}
			eventually(t, 30*time.Second, "completed source waited for the other source group", func() bool { _, err := connected.State().Catalog.Provider(fast); return err == nil })
			select {
			case err := <-result:
				t.Fatalf("run completed before slow source answered: %v", err)
			default:
			}
			unblock.Do(func() { close(release) })
			select {
			case err := <-result:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("run did not join both source groups")
			}
			activity := connected.Status().SourceActivities
			if len(activity) != 1 || activity[0].Source != sources.ProvidersID || !activity[0].Attempted || activity[0].Eligibility != sources.EligibilityUnknown {
				t.Fatalf("mixed-source run lost provider activity or inferred eligibility from a layer: %+v", activity)
			}
			for _, id := range []catalogs.ProviderID{"provider", "metadata"} {
				if _, err := connected.State().Catalog.Provider(id); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSourceAcquisitionSchedulesWithoutProviderRole(t *testing.T) {
	observation := acquisitionSourceObservation(t, false)
	var calls atomic.Int32
	metadata := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		calls.Add(1)
		return []sources.Observation{observation}, nil
	})
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithSourceAcquirer(metadata), WithAcquisitionEnabled(true), WithAcquisitionInterval(0), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
	eventually(t, 30*time.Second, "source-only startup acquisition did not complete", func() bool { return connected.Status().LastRunID != "" })
	if calls.Load() != 1 {
		t.Fatalf("startup calls = %d", calls.Load())
	}
	if _, err := connected.State().Catalog.Provider("metadata"); err != nil {
		t.Fatal(err)
	}
}

func TestSourceAcquisitionRetainsReceiptsAndReportsPartialFailure(t *testing.T) {
	store := storage.NewMemory()
	observation := acquisitionSourceObservation(t, true)
	metadata := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		return []sources.Observation{observation}, nil
	})
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithSourceAcquirer(metadata), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	report, err := connected.Sync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if report.Health != HealthDegraded || connected.Status().AcquisitionHealth != HealthDegraded || len(report.SourceObservations) != 1 || report.SourceObservations[0].ObservationID != observation.ID {
		t.Fatalf("partial source report = %+v", report)
	}
	status := connected.Status()
	if len(status.SourceObservations) != 1 {
		t.Fatal("runtime status omitted source receipt")
	}
	status.SourceObservations[0].ObservationID = "mutated"
	if connected.Status().SourceObservations[0].ObservationID != observation.ID {
		t.Fatal("caller mutation changed runtime source status")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, link := range accepted.Manifest.SourceObservations {
		if link.ObservationID == observation.ID && link.EvidenceChecksum == observation.EvidenceChecksum {
			found = true
		}
	}
	if !found || !accepted.Manifest.Degraded {
		t.Fatal("generation lost the original partial source receipt")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("restart changed accepted generation")
	}
	if _, err := reopened.State().Catalog.Provider("metadata"); err != nil {
		t.Fatal(err)
	}
}

func TestSourceAcquisitionCancellationAndRejectionPreserveAcceptedState(t *testing.T) {
	for _, scenario := range []string{"failure", "cancel", "forbidden-provider", "invalid-receipt"} {
		t.Run(scenario, func(t *testing.T) {
			observation := acquisitionSourceObservation(t, false)
			if scenario == "forbidden-provider" {
				observation = manualProviderObservation(t, 10, time.Now().UTC())
			}
			if scenario == "invalid-receipt" {
				observation.EvidenceChecksum = "invalid"
			}
			entered := make(chan struct{})
			metadata := sourceAcquirerFunc(func(ctx context.Context, _ SourceAcquisitionRequest) ([]sources.Observation, error) {
				close(entered)
				switch scenario {
				case "failure":
					return nil, stderrors.New("source unavailable")
				case "cancel":
					<-ctx.Done()
					return []sources.Observation{observation}, nil
				}
				return []sources.Observation{observation}, nil
			})
			connected := openTestRuntime(t, WithCatalogSource("embedded"), WithSourceAcquirer(metadata), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
			before := connected.State()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := connected.Sync(ctx); result <- err }()
			<-entered
			if scenario == "cancel" {
				cancel()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("invalid source result succeeded")
				}
			case <-time.After(30 * time.Second):
				t.Fatal("source acquisition did not terminate")
			}
			if connected.State().GenerationID != before.GenerationID {
				t.Fatal("failed source acquisition replaced accepted state")
			}
		})
	}
}

func TestSourceAcquisitionStartupRechecksDespiteFreshProviderEvidence(t *testing.T) {
	layer := testProviderLayer(t, "provider", "model", "Provider", time.Now().UTC())
	store := storage.NewMemory()
	directory := privateRuntimeDirectory(t)
	provider := windowAcquirerFunc(func(context.Context, AcquisitionRequest) (AcquisitionResult, error) {
		return AcquisitionResult{Layers: []ProviderLayer{layer}}, nil
	})
	first := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquirer(provider), WithClientOptions(starmap.WithCatalogStore(store)))
	if _, err := first.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	observation := acquisitionSourceObservation(t, false)
	var calls atomic.Int32
	metadata := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		calls.Add(1)
		return []sources.Observation{observation}, nil
	})
	connected := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithSourceAcquirer(metadata), WithAcquisitionEnabled(true), WithAcquisitionInterval(24*time.Hour), WithClientOptions(starmap.WithCatalogStore(store)))
	eventually(t, 30*time.Second, "fresh provider evidence delayed missing metadata", func() bool { return connected.Status().AcquisitionHealth != HealthUnknown })
	if calls.Load() != 1 {
		t.Fatalf("startup calls = %d", calls.Load())
	}
	if _, err := connected.State().Catalog.Provider("metadata"); err != nil {
		t.Fatal(err)
	}
}

func TestSourceAcquisitionLeaseLossRefusesPublication(t *testing.T) {
	observation := acquisitionSourceObservation(t, false)
	entered, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	metadata := sourceAcquirerFunc(func(ctx context.Context, _ SourceAcquisitionRequest) ([]sources.Observation, error) {
		close(entered)
		select {
		case <-release:
			return []sources.Observation{observation}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	leases := &stubLeaseStore{}
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithSourceAcquirer(metadata), WithLeaseStore(leases), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
	before := connected.State().GenerationID
	completed := make(chan error, 1)
	go func() { _, err := connected.Sync(t.Context()); completed <- err }()
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("source did not start")
	}
	leases.bumpEpoch()
	if err := connected.lease.renewOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	unblock.Do(func() { close(release) })
	select {
	case err := <-completed:
		if err == nil {
			t.Fatal("stale source lease published")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("source did not finish")
	}
	if connected.State().GenerationID != before {
		t.Fatal("source changed accepted generation after lease loss")
	}
}
