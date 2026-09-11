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

func originTestConfig() OriginConfig {
	return OriginConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() permission.ClockReading {
		return permission.ClockReading{Time: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), Known: true}
	}}
}

func TestOriginPublicationBindsJournalBeforeAmbiguousCommit(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	store := &retentionAmbiguousStore{Memory: storage.NewMemory()}
	opts := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding), WithAuthorityOrigin(store, originTestConfig())}
	connected := openTestRuntime(t, opts...)
	before := connected.State()
	store.ambiguous.Store(true)
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err == nil {
		t.Fatal("lost commit reply did not return an error")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Manifest.AuthorityHead.Sequence != 2 {
		t.Fatalf("sequence = %d", accepted.Manifest.AuthorityHead.Sequence)
	}
	pending, err := connected.store.loadInputPublication()
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil || pending.GenerationID != accepted.Manifest.GenerationID || pending.PayloadChecksum != accepted.Manifest.Payload.Checksum || pending.ExpectedID != before.GenerationID {
		t.Fatalf("journal does not bind the accepted authority: %+v", pending)
	}
	if connected.State().GenerationID != before.GenerationID {
		t.Fatal("ambiguous commit changed active catalog")
	}
	receipt, err := connected.ReadPermission(t.Context())
	if err != nil || receipt.Head != accepted.Manifest.AuthorityHead {
		t.Fatalf("issuer concealed the durable head: %+v, %v", receipt.Head, err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	recovered := openTestRuntime(t, opts...)
	if recovered.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("recovery minted a different authority identity")
	}
	retained, err := recovered.store.loadProviders()
	if err != nil || len(retained) != 1 {
		t.Fatalf("retained observations = %d, %v", len(retained), err)
	}
	if pending, err := recovered.store.loadInputPublication(); err != nil || pending != nil {
		t.Fatalf("pending recovery: %+v, %v", pending, err)
	}
}

func TestOriginPublicationRetainsIdentityOnRestartAndRebuild(t *testing.T) {
	store := storage.NewMemory()
	opts := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig())}
	connected := openTestRuntime(t, opts...)
	first, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.AuthorityHead.Sequence != 1 {
		t.Fatal("initial authority is missing")
	}
	state, err := connected.rebuild(t.Context(), connected.lease.epoch())
	if err != nil || state.GenerationID != first.Manifest.GenerationID {
		t.Fatalf("unchanged rebuild changed identity: %s, %v", state.GenerationID, err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	recovered := openTestRuntime(t, opts...)
	if recovered.State().GenerationID != first.Manifest.GenerationID {
		t.Fatal("restart minted a new authority sequence")
	}
	if _, err := recovered.Client().Update(t.Context(), func(_ context.Context, c *catalogs.Catalog) (*starmap.Candidate, error) {
		return starmap.NewCandidate(c, starmap.CandidateEvidence{})
	}); !errors.IsConflict(err) {
		t.Fatalf("direct publication bypassed origin: %v", err)
	}
}
