package runtime

import (
	"context"
	stderrors "errors"
	"math"
	"reflect"
	"testing"
)

func fleetTestPublication(t *testing.T) FleetPublication {
	t.Helper()
	generation := aliasGeneration(t, "fleet-generation")
	data := []byte(`{"version":1,"publisher_id":"deployment"}`)
	return FleetPublication{Generation: generation,
		Recovery: FleetRecovery{GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
			Checksum: fleetRecoveryChecksum(data), Data: data},
		Grant: Lease{Holder: "replica", SessionID: "process-session", Epoch: 7,
			Identity: FleetIdentity{DeploymentID: "deployment", RecoveryEpoch: 3, BackendID: "approved-process"}}}
}

func TestFleetPublicationRejectsIncompleteAndForeignIdentity(t *testing.T) {
	valid := fleetTestPublication(t)
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		change func(*FleetPublication)
	}{
		{"holder", func(p *FleetPublication) { p.Grant.Holder = "" }},
		{"session", func(p *FleetPublication) { p.Grant.SessionID = "" }},
		{"epoch", func(p *FleetPublication) { p.Grant.Epoch = 0 }},
		{"deployment", func(p *FleetPublication) { p.Grant.Identity.DeploymentID = "" }},
		{"recovery-epoch", func(p *FleetPublication) { p.Grant.Identity.RecoveryEpoch = 0 }},
		{"backend", func(p *FleetPublication) { p.Grant.Identity.BackendID = "" }},
		{"foreign-predecessor", func(p *FleetPublication) { p.Expected = valid.nextHead(); p.Expected.Identity.DeploymentID = "other" }},
		{"restored-predecessor", func(p *FleetPublication) { p.Expected = valid.nextHead(); p.Expected.Identity.RecoveryEpoch-- }},
		{"different-process", func(p *FleetPublication) {
			p.Expected = valid.nextHead()
			p.Expected.Identity.BackendID = "old-process"
		}},
		{"revision-overflow", func(p *FleetPublication) { p.Expected = valid.nextHead(); p.Expected.Revision = math.MaxUint64 }},
		{"partial-head", func(p *FleetPublication) { p.Expected.GenerationID = "generation" }},
		{"malformed-digest", func(p *FleetPublication) { p.Expected = valid.nextHead(); p.Expected.RecoveryChecksum = "invalid" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			candidate := valid
			scenario.change(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("publication accepted incomplete or foreign ownership evidence")
			}
		})
	}
}

func TestFleetHeadDistinguishesInputsForUnchangedCatalog(t *testing.T) {
	first := fleetTestPublication(t)
	next := first
	next.Expected = first.nextHead()
	next.Recovery.Data = []byte(`{"version":1,"publisher_id":"other-retained-inputs"}`)
	next.Recovery.Checksum = fleetRecoveryChecksum(next.Recovery.Data)
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	if next.nextHead().GenerationID != first.nextHead().GenerationID || next.nextHead() == first.nextHead() || next.nextHead().Revision != 2 {
		t.Fatal("the head failed to distinguish recovery inputs for an unchanged catalog")
	}
	snapshot := FleetSnapshot{Head: next.nextHead(), Publication: next}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	snapshot.Head.RecoveryChecksum = first.Recovery.Checksum
	if err := snapshot.Validate(); err == nil {
		t.Fatal("snapshot accepted recovery inputs from an earlier publication")
	}
}

// fleetRecordingStore tests dispatch, not the backend transaction contract.
// The real adapter must separately prove atomic expiry and head comparison.
type fleetRecordingStore struct {
	FleetStore
	attempts  []FleetPublication
	err       error
	wrongHead bool
}

func (s *fleetRecordingStore) CommitPublication(_ context.Context, p FleetPublication) (FleetHead, error) {
	s.attempts = append(s.attempts, p)
	head := p.nextHead()
	if s.wrongHead {
		head.Revision++
	}
	return head, s.err
}

func TestFleetCommitPreservesOriginalGrantAndInputsOnRetry(t *testing.T) {
	backend := &fleetRecordingStore{err: stderrors.New("ambiguous backend response")}
	store := &fleetCommitStore{FleetStore: backend}
	p := fleetTestPublication(t)
	ctx, attempt, err := store.prepareWithPin(t.Context(), p.Grant, p.Expected, layerSet{publisherID: "deployment"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(ctx, p.Generation, ""); !stderrors.Is(err, backend.err) {
		t.Fatalf("first attempt: %v", err)
	}
	if attempt.result() != (FleetHead{}) {
		t.Fatal("an ambiguous response became an accepted publication")
	}
	backend.err = nil
	if err := store.Commit(ctx, p.Generation, ""); err != nil {
		t.Fatal(err)
	}
	if len(backend.attempts) != 2 || !reflect.DeepEqual(backend.attempts[0], backend.attempts[1]) {
		t.Fatal("retry changed the original grant, predecessor, or recovery inputs")
	}
	if err := (FleetSnapshot{Head: attempt.result(), Publication: backend.attempts[1]}).Validate(); err != nil {
		t.Fatal(err)
	}
	other := aliasGeneration(t, "different-generation")
	if err := store.Commit(ctx, other, ""); err == nil || len(backend.attempts) != 2 {
		t.Fatal("retry substituted a new generation")
	}
}

func TestFleetCommitRefusesUnownedContextsAndWrongPredecessor(t *testing.T) {
	backend := &fleetRecordingStore{}
	store := &fleetCommitStore{FleetStore: backend}
	p := fleetTestPublication(t)
	ctx, _, err := store.prepareWithPin(t.Context(), p.Grant, p.Expected, layerSet{publisherID: "deployment"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), p.Generation, ""); err == nil {
		t.Fatal("direct client publication bypassed the fleet grant")
	}
	if err := store.Commit(ctx, p.Generation, "different-predecessor"); err == nil {
		t.Fatal("publication used a different predecessor")
	}
	other := &fleetCommitStore{FleetStore: backend}
	if err := other.Commit(ctx, p.Generation, ""); err == nil {
		t.Fatal("publication context crossed stores")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Commit(canceled, p.Generation, ""); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled commit: %v", err)
	}
	if len(backend.attempts) != 0 {
		t.Fatal("a refused request reached the backend")
	}
}

func TestFleetCommitRejectsMismatchedBackendReceipt(t *testing.T) {
	backend := &fleetRecordingStore{wrongHead: true}
	store := &fleetCommitStore{FleetStore: backend}
	p := fleetTestPublication(t)
	ctx, attempt, err := store.prepareWithPin(t.Context(), p.Grant, p.Expected, layerSet{publisherID: "deployment"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(ctx, p.Generation, ""); err == nil {
		t.Fatal("publication accepted a different backend receipt")
	}
	if attempt.result() != (FleetHead{}) {
		t.Fatal("an invalid receipt became the selected publication")
	}
}
