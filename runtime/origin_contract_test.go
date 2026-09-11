package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOriginPublicationRequiresExplicitOrdinaryBootstrap(t *testing.T) {
	store := storage.NewMemory()
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := starmap.NewCandidate(empty, starmap.CandidateEvidence{}, starmap.WithCandidateGenerationID("ordinary"))
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := client.PrepareGeneration(t.Context(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Activate(t.Context(), ordinary); err != nil {
		t.Fatal(err)
	}
	base := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0)}
	if r, err := Open(t.Context(), append(base, WithAuthorityOrigin(store, originTestConfig()))...); err == nil {
		_ = r.Close()
		t.Fatal("ordinary store was adopted without authorization")
	}
	retained, err := store.Current(t.Context())
	if err != nil || retained.Manifest.GenerationID != "ordinary" {
		t.Fatalf("refusal changed the store: %v", err)
	}
	config := originTestConfig()
	config.Bootstrap = true
	blocked := append(base, WithAuthorityOrigin(store, config), WithClientOptions(starmap.WithPublicationGuard(func(context.Context) error {
		return &errors.ConflictError{Resource: "operator publication policy", Message: "publication is disabled"}
	})))
	if r, err := Open(t.Context(), blocked...); err == nil {
		_ = r.Close()
		t.Fatal("publication guard allowed bootstrap")
	}
	retained, err = store.Current(t.Context())
	if err != nil || retained.Manifest.GenerationID != "ordinary" {
		t.Fatalf("bootstrap bypassed the publication guard: %s, %v", retained.Manifest.GenerationID, err)
	}
	connected := openTestRuntime(t, append(base, WithAuthorityOrigin(store, config))...)
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Manifest.AuthorityHead.Sequence != 1 || accepted.Manifest.Payload != ordinary.Manifest.Payload {
		t.Fatal("bootstrap changed the catalog or omitted authority")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	recovered := openTestRuntime(t, append(base, WithAuthorityOrigin(store, config))...)
	if recovered.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("bootstrap reset an established authority")
	}
	config.AuthorityID = "other"
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	if r, err := Open(t.Context(), append(base, WithAuthorityOrigin(store, config))...); err == nil {
		_ = r.Close()
		t.Fatal("changed authority was accepted")
	}
}

func TestOriginPublicationRejectsInputsAfterFailedCommit(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	opts := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding), WithAuthorityOrigin(store, originTestConfig())}
	connected := openTestRuntime(t, opts...)
	before := connected.State()
	store.reject.Store(true)
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err == nil {
		t.Fatal("rejected commit succeeded")
	}
	if connected.State().GenerationID != before.GenerationID {
		t.Fatal("rejected catalog became active")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	recovered := openTestRuntime(t, opts...)
	retained, err := recovered.store.loadProviders()
	if err != nil || len(retained) != 0 {
		t.Fatalf("rejected observations survived: %d, %v", len(retained), err)
	}
	if recovered.State().GenerationID != before.GenerationID {
		t.Fatal("rejected inputs changed restart identity")
	}
}

func TestOriginPublicationRequiresQualifiedClockForReceipts(t *testing.T) {
	config := originTestConfig()
	config.Clock = func() permission.ClockReading { return permission.ClockReading{} }
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(storage.NewMemory(), config))
	if _, err := connected.ReadPermission(t.Context()); err == nil {
		t.Fatal("unknown clock issued a receipt")
	}
	if connected.Catalog() == nil {
		t.Fatal("clock uncertainty removed catalog diagnostics")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.ReadPermission(t.Context()); err == nil {
		t.Fatal("closed origin issued a receipt")
	}
}

func TestOriginPublicationRefusesSubscriberConfigurationBeforeStorageReads(t *testing.T) {
	store := storage.NewMemory()
	_, err := Open(t.Context(), WithCatalogSource("starmap"), WithSourceURL("https://authority.example"), WithSourceStartupPolicy(string(StartupRequireAuthority)), WithAuthorityOrigin(store, originTestConfig()))
	if err == nil {
		t.Fatal("subscriber became an origin")
	}
	if _, err := store.Current(t.Context()); !errors.IsNotFound(err) {
		t.Fatalf("invalid configuration wrote storage: %v", err)
	}
}
