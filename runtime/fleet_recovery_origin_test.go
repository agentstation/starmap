package runtime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func recoveryOriginPublication(t *testing.T, publication FleetPublication) FleetPublication {
	t.Helper()
	manifest, err := catalogManifestChecksum(publication.Generation)
	if err != nil {
		t.Fatal(err)
	}
	publication.RecoveryOrigin = &FleetRecoveryOrigin{Version: FleetRecoveryOriginVersion, Identity: publication.Grant.Identity,
		OperationID: "closed-local-import", AcceptedDecisionSHA256: fleetRecoveryChecksum([]byte("accepted-decision")),
		ClosedImportSHA256: fleetRecoveryChecksum([]byte("closed-import")), GenerationManifestSHA256: manifest,
		RecoverySHA256: publication.Recovery.Checksum}
	publication.Grant = Lease{}
	return publication
}

func TestFleetRecoveryOriginRetainsExactCatalogAndInputBytes(t *testing.T) {
	original := fleetTestPublication(t)
	if err := original.ValidateRefreshPublication(); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(original)
	if err != nil || bytes.Contains(wire, []byte("recovery_origin")) {
		t.Fatal("ordinary publication encoding changed", err)
	}
	publication := recoveryOriginPublication(t, original)
	if err := publication.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := publication.ValidateRefreshPublication(); err == nil {
		t.Fatal("recovery import passed ordinary refresh validation")
	}
	head := publication.nextHead()
	if head.Identity != publication.RecoveryOrigin.Identity || head.Revision != 1 || head.GenerationID != original.Generation.Manifest.GenerationID || head.RecoveryChecksum != original.Recovery.Checksum {
		t.Fatal("recovery origin changed the selected head")
	}
	snapshot := FleetSnapshot{Head: head, Publication: publication}
	wire, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var decoded FleetSnapshot
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Publication.Generation, original.Generation) || !reflect.DeepEqual(decoded.Publication.Recovery, original.Recovery) || *decoded.Publication.RecoveryOrigin != *publication.RecoveryOrigin || decoded.Publication.Grant != (Lease{}) {
		t.Fatal("encoding changed original bytes or created a refresh grant")
	}
	next := publication
	next.Expected = head
	if err := next.Validate(); err != nil || next.nextHead().Revision != 2 {
		t.Fatal("exact predecessor did not advance once", err)
	}
}

func TestFleetRecoveryOriginRejectsIncompleteOrChangedEvidence(t *testing.T) {
	valid := recoveryOriginPublication(t, fleetTestPublication(t))
	for _, scenario := range []struct {
		name   string
		change func(*FleetPublication)
	}{
		{"version", func(p *FleetPublication) { p.RecoveryOrigin.Version++ }},
		{"operation-empty", func(p *FleetPublication) { p.RecoveryOrigin.OperationID = "" }},
		{"operation-whitespace", func(p *FleetPublication) { p.RecoveryOrigin.OperationID += " " }},
		{"operation-control", func(p *FleetPublication) { p.RecoveryOrigin.OperationID = "bad\noperation" }},
		{"operation-utf8", func(p *FleetPublication) { p.RecoveryOrigin.OperationID = string([]byte{0xff}) }},
		{"operation-limit", func(p *FleetPublication) { p.RecoveryOrigin.OperationID = strings.Repeat("x", deploymentIDMaxBytes+1) }},
		{"decision", func(p *FleetPublication) { p.RecoveryOrigin.AcceptedDecisionSHA256 = "invalid" }},
		{"closed-import", func(p *FleetPublication) { p.RecoveryOrigin.ClosedImportSHA256 = "" }},
		{"manifest", func(p *FleetPublication) { p.RecoveryOrigin.GenerationManifestSHA256 = "invalid" }},
		{"recovery", func(p *FleetPublication) { p.RecoveryOrigin.RecoverySHA256 = "invalid" }},
		{"changed-manifest", func(p *FleetPublication) {
			p.Generation.Manifest.GeneratedAt = p.Generation.Manifest.GeneratedAt.Add(time.Second)
		}},
		{"changed-inputs", func(p *FleetPublication) {
			p.Recovery.Data = append(bytes.Clone(p.Recovery.Data), '\n')
			p.Recovery.Checksum = fleetRecoveryChecksum(p.Recovery.Data)
		}},
		{"deployment", func(p *FleetPublication) { p.RecoveryOrigin.Identity.DeploymentID = "" }},
		{"recovery-epoch", func(p *FleetPublication) { p.RecoveryOrigin.Identity.RecoveryEpoch = 0 }},
		{"backend", func(p *FleetPublication) { p.RecoveryOrigin.Identity.BackendID = "" }},
		{"foreign-predecessor", func(p *FleetPublication) { p.Expected = valid.nextHead(); p.Expected.Identity.DeploymentID = "foreign" }},
		{"revision-overflow", func(p *FleetPublication) { p.Expected = valid.nextHead(); p.Expected.Revision = math.MaxUint64 }},
		{"no-ownership", func(p *FleetPublication) { p.RecoveryOrigin = nil }},
		{"grant-holder", func(p *FleetPublication) { p.Grant.Holder = "fake" }},
		{"grant-session", func(p *FleetPublication) { p.Grant.SessionID = "fake" }},
		{"grant-epoch", func(p *FleetPublication) { p.Grant.Epoch = 1 }},
		{"grant-identity", func(p *FleetPublication) { p.Grant.Identity.DeploymentID = "fake" }},
		{"grant-expiry", func(p *FleetPublication) { p.Grant.ExpiresAt = time.Now() }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			changed := valid
			origin := *valid.RecoveryOrigin
			changed.RecoveryOrigin = &origin
			changed.Generation = valid.Generation.Copy()
			scenario.change(&changed)
			if err := changed.Validate(); err == nil {
				t.Fatal("accepted incomplete or changed recovery origin")
			}
		})
	}
}

func TestFleetRecoveryOriginRefusedByNormalCommitStores(t *testing.T) {
	publication := recoveryOriginPublication(t, fleetTestPublication(t))
	backend := newFleetRuntimeBackend(t)
	session := &fleetTestSession{backend: backend, session: "refresh"}
	head := backend.head
	if _, err := session.CommitPublication(t.Context(), publication); err == nil || backend.head != head {
		t.Fatal("ordinary transactional commit accepted recovery evidence")
	}
	recording := &fleetRecordingStore{}
	if _, err := recording.CommitPublication(t.Context(), publication); err == nil || len(recording.attempts) != 0 {
		t.Fatal("ordinary recording adapter dispatched recovery evidence")
	}
}

func TestFleetRecoveryOriginReplayAndAdoptionRemainPassive(t *testing.T) {
	snapshot, _ := fleetValidationFixture(t)
	original := snapshot.Publication
	snapshot.Publication = recoveryOriginPublication(t, original)
	snapshot.Head = snapshot.Publication.nextHead()
	before, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	source := newStubSource("retained-source")
	leases := &stubLeaseStore{}
	clockCalls := 0
	directory := filepath.Join(t.TempDir(), "must-not-be-created")
	opts := []Option{WithSource(source), WithLeaseStore(leases), WithStateDirectory(directory), WithClock(func() time.Time {
		clockCalls++
		return time.Now()
	})}
	if err := ValidateFleetRecovery(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFleetReplay(t.Context(), snapshot, opts...); err != nil {
		t.Fatal(err)
	}
	if source.readCount() != 0 || leases.acquireCount() != 0 || clockCalls != 0 {
		t.Fatal("recovery replay started external roles")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("recovery replay created runtime state", err)
	}
	after, err := json.Marshal(snapshot)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("replay changed retained recovery origin", err)
	}
	previous := snapshot.Head
	snapshot.Head.Identity.RecoveryEpoch++
	snapshot.Head.Identity.BackendID = "restored-again"
	snapshot.Adoption = &FleetAdoption{Previous: previous, Receipt: fleetRecoveryChecksum([]byte("adoption"))}
	if err := ValidateFleetReplay(t.Context(), snapshot, opts...); err != nil {
		t.Fatal("explicit adoption lost the original recovery origin", err)
	}
	if !reflect.DeepEqual(snapshot.Publication.Generation, original.Generation) || !reflect.DeepEqual(snapshot.Publication.Recovery, original.Recovery) || snapshot.Publication.RecoveryOrigin.Identity != previous.Identity {
		t.Fatal("adoption changed original facts or recovery identity")
	}
	snapshot.Head.RecoveryChecksum = fleetRecoveryChecksum([]byte("changed-inputs"))
	if err := snapshot.Validate(); err == nil {
		t.Fatal("adoption changed selected reconstruction inputs")
	}
}

func TestFleetRecoveryOriginRequiresRealOwnershipForLaterRefresh(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	originalHead := backend.head
	original := backend.snapshots[originalHead].Publication
	recovered := recoveryOriginPublication(t, original)
	backend.snapshots[originalHead] = FleetSnapshot{Head: originalHead, Publication: recovered}
	busy := &fleetTestSession{backend: backend, session: "busy-process"}
	occupied, err := busy.AcquireLease(t.Context(), "busy-holder", LeaseTTL)
	if err != nil {
		t.Fatal(err)
	}
	r := openFleetRuntime(t, backend, "follower", privateRuntimeDirectory(t))
	if r.lease.status() != leaseLost || backend.head != originalHead || r.fleetPublicationGrant != (Lease{}) || r.permissions.required.Version != 0 || r.State().AuthorityHead != (catalogs.CatalogAuthorityHead{}) || r.config.origin != nil {
		t.Fatal("recovery origin became refresh or serving authority")
	}
	if err := busy.Release(t.Context(), occupied); err != nil {
		t.Fatal(err)
	}
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	current := backend.snapshots[backend.head]
	retained := backend.snapshots[originalHead]
	active := backend.lease
	backend.mu.Unlock()
	if current.Head.Revision <= originalHead.Revision || current.Publication.RecoveryOrigin != nil || !sameLeaseGrant(current.Publication.Grant, active) || current.Publication.Grant.Epoch <= occupied.Epoch {
		t.Fatal("later refresh reused recovery evidence as a grant")
	}
	if err := current.Publication.ValidateRefreshPublication(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current.Publication.Generation.Payload, original.Generation.Payload) || !reflect.DeepEqual(retained.Publication, recovered) {
		t.Fatal("ownership publication changed recovered facts or original evidence")
	}
}

func TestFleetRecoveryOriginReplayCancellation(t *testing.T) {
	snapshot, _ := fleetValidationFixture(t)
	snapshot.Publication = recoveryOriginPublication(t, snapshot.Publication)
	snapshot.Head = snapshot.Publication.nextHead()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ValidateFleetReplay(ctx, snapshot); err == nil {
		t.Fatal("canceled recovery replay succeeded")
	}
}
