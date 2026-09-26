package runtime

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// fleetTestBackend exercises runtime composition with an atomic in-process store.
// Starport must separately qualify its native transaction and expiry contract.
type fleetTestBackend struct {
	mu           sync.Mutex
	identity     FleetIdentity
	head         FleetHead
	snapshots    map[FleetHead]FleetSnapshot
	generations  map[string]catalogs.Generation
	lease        Lease
	epoch        uint64
	acquisitions int
}

type fleetTestSession struct {
	backend *fleetTestBackend
	session string
}

func newFleetRuntimeBackend(t *testing.T) *fleetTestBackend {
	t.Helper()
	p := fleetTestPublication(t)
	embedded, manifest, err := bootstrap.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	layers := layerSet{publisherID: "deployment", embedded: starmap.CatalogState{Catalog: embedded,
		GenerationID: manifest.GenerationID, PayloadChecksum: manifest.Payload.Checksum, GeneratedAt: manifest.GeneratedAt},
		source: &sourceLayer{Identity: "fleet-source", GenerationID: p.Generation.Manifest.GenerationID,
			Payload: p.Generation.Payload, Checksum: p.Generation.Manifest.Payload.Checksum, Manifest: &p.Generation.Manifest,
			PublishedAt: p.Generation.Manifest.GeneratedAt}}
	layers.sourceConfiguration, err = describeSources(defaults())
	if err != nil {
		t.Fatal(err)
	}
	p.Recovery.Data, err = encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Recovery.Checksum = fleetRecoveryChecksum(p.Recovery.Data)
	snapshot := FleetSnapshot{Head: p.nextHead(), Publication: p}
	return &fleetTestBackend{identity: p.Grant.Identity, head: snapshot.Head, epoch: p.Grant.Epoch,
		snapshots: map[FleetHead]FleetSnapshot{snapshot.Head: snapshot}, generations: map[string]catalogs.Generation{p.Generation.Manifest.GenerationID: p.Generation}}
}

func (s *fleetTestSession) CurrentHead(ctx context.Context) (FleetHead, error) {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return FleetHead{}, err
	}
	if b.head == (FleetHead{}) {
		return FleetHead{}, &errors.NotFoundError{Resource: "fleet publication", ID: "current"}
	}
	return b.head, nil
}

func (s *fleetTestSession) CurrentPublication(ctx context.Context) (FleetSnapshot, error) {
	head, err := s.CurrentHead(ctx)
	if err != nil {
		return FleetSnapshot{}, err
	}
	return s.Publication(ctx, head)
}

func (s *fleetTestSession) Publication(ctx context.Context, head FleetHead) (FleetSnapshot, error) {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return FleetSnapshot{}, err
	}
	snapshot, ok := b.snapshots[head]
	if !ok {
		return FleetSnapshot{}, &errors.NotFoundError{Resource: "fleet publication", ID: head.GenerationID}
	}
	snapshot.Publication.Generation = snapshot.Publication.Generation.Copy()
	snapshot.Publication.Recovery.Data = bytes.Clone(snapshot.Publication.Recovery.Data)
	return snapshot, nil
}

func (s *fleetTestSession) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	generation, ok := b.generations[id]
	if !ok {
		return catalogs.Generation{}, &errors.NotFoundError{Resource: "generation", ID: id}
	}
	return generation.Copy(), nil
}

func (s *fleetTestSession) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	head := b.generations[b.head.GenerationID].Manifest.AuthorityHead
	if head == (catalogs.CatalogAuthorityHead{}) {
		return head, &errors.NotFoundError{Resource: "authority head", ID: "current"}
	}
	return head, nil
}

func (s *fleetTestSession) AcquireLease(ctx context.Context, holder string, ttl time.Duration) (Lease, error) {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Lease{}, err
	}
	b.acquisitions++
	if b.lease.Holder != "" && time.Now().Before(b.lease.ExpiresAt) {
		if b.lease.Holder == holder && b.lease.SessionID != s.session {
			return Lease{}, &errors.ConfigError{Component: "fleet identity", Message: "the instance identity is already active"}
		}
		if b.lease.Holder != holder || b.lease.SessionID != s.session {
			return Lease{}, fleetConflict("another process owns refresh")
		}
		return b.lease, nil
	}
	b.epoch++
	b.lease = Lease{Holder: holder, SessionID: s.session, Epoch: b.epoch, Identity: b.identity, ExpiresAt: time.Now().Add(ttl)}
	return b.lease, nil
}

func (s *fleetTestSession) Renew(ctx context.Context, lease Lease, ttl time.Duration) (Lease, error) {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Lease{}, err
	}
	if !sameLeaseGrant(b.lease, lease) || !time.Now().Before(b.lease.ExpiresAt) {
		return Lease{}, fleetConflict("the original grant expired")
	}
	b.lease.ExpiresAt = time.Now().Add(ttl)
	return b.lease, nil
}

func (s *fleetTestSession) Release(ctx context.Context, lease Lease) error {
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if sameLeaseGrant(b.lease, lease) {
		b.lease.Holder = ""
		b.lease.ExpiresAt = time.Time{}
	}
	return nil
}

func (s *fleetTestSession) CommitPublication(ctx context.Context, p FleetPublication) (FleetHead, error) {
	if err := p.Validate(); err != nil {
		return FleetHead{}, err
	}
	b := s.backend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return FleetHead{}, err
	}
	if prior, ok := b.snapshots[p.nextHead()]; ok && reflect.DeepEqual(prior.Publication, p) {
		return prior.Head, nil
	}
	if b.head != p.Expected || !sameLeaseGrant(b.lease, p.Grant) || !time.Now().Before(b.lease.ExpiresAt) || p.Grant.Identity != b.identity {
		return FleetHead{}, fleetConflict("the grant or predecessor changed")
	}
	if prior, ok := b.generations[p.Generation.Manifest.GenerationID]; ok && !reflect.DeepEqual(prior, p.Generation) {
		return FleetHead{}, fleetConflict("the generation is already bound to different bytes")
	}
	p.Generation = p.Generation.Copy()
	p.Recovery.Data = bytes.Clone(p.Recovery.Data)
	b.head = p.nextHead()
	b.snapshots[b.head] = FleetSnapshot{Head: b.head, Publication: p}
	b.generations[p.Generation.Manifest.GenerationID] = p.Generation
	return b.head, nil
}

func openFleetRuntime(t *testing.T, b *fleetTestBackend, identity, directory string, options ...Option) *Runtime {
	t.Helper()
	session := &fleetTestSession{backend: b, session: identity + "-process"}
	selected := []Option{WithFleetStore(session), WithSchedulerIdentity(identity), WithStateDirectory(directory), WithSource(newStubSource("fleet-source")), withScheduleTimer(newStubScheduleTimer().after)}
	return openTestRuntime(t, append(selected, options...)...)
}

func TestFleetRuntimeRecoversInputsWithoutLeaderDirectory(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	leaderDirectory := privateRuntimeDirectory(t)
	leader := openFleetRuntime(t, backend, "leader", leaderDirectory)
	at := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", at, false)); err != nil {
		t.Fatal(err)
	}
	follower := openFleetRuntime(t, backend, "follower", privateRuntimeDirectory(t))
	if follower.lease.status() != leaseLost {
		t.Fatal("follower acquired another process's grant")
	}
	if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "second", at.Add(time.Minute), true)); err != nil {
		t.Fatal(err)
	}
	if err := follower.RefreshFleet(t.Context()); err != nil {
		t.Fatal(err)
	}
	if follower.State().GenerationID != leader.State().GenerationID {
		t.Fatal("missed-event recovery did not activate the durable head")
	}
	if err := leader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(leaderDirectory); err != nil {
		t.Fatal(err)
	}
	if _, err := follower.PublishObservations(t.Context(), manualTestObservation(t, "third", at.Add(2*time.Minute), true)); err != nil {
		t.Fatal(err)
	}
	if err := follower.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openFleetRuntime(t, backend, "replacement", privateRuntimeDirectory(t))
	provider, err := restarted.Catalog().Provider("manual-provider")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "second", "third"} {
		if provider.Models[id] == nil {
			t.Errorf("recovery lost model %q", id)
		}
	}
	status, ok := restarted.FleetStatus()
	if !ok || !status.ReplayReady || status.Head.Revision != 4 {
		t.Fatalf("fleet state: %+v", status)
	}
	if _, err := restarted.Client().Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		t.Fatal("unguarded client write reached candidate production")
		return nil, nil
	}); err == nil {
		t.Fatal("direct client write bypassed fleet ownership")
	}
}

func TestFleetRuntimeIncompatibleReplayRemainsFollower(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	follower := openFleetRuntime(t, backend, "different-policy", privateRuntimeDirectory(t), WithAcquisitionSources())
	status, ok := follower.FleetStatus()
	if !ok || status.ReplayReady || follower.lease.status() != leaseLost {
		t.Fatalf("incompatible follower: %+v", status)
	}
	if follower.State().GenerationID != backend.head.GenerationID {
		t.Fatal("incompatible acquisition policy hid a supported accepted catalog")
	}
	if _, err := follower.PublishObservations(t.Context(), manualTestObservation(t, "forbidden", time.Now().UTC(), false)); err == nil {
		t.Fatal("incompatible replica acquired refresh ownership")
	}
	backend.mu.Lock()
	takes := backend.acquisitions
	backend.mu.Unlock()
	if takes != 0 {
		t.Fatal("incompatible replica tried to acquire the lease")
	}
}

func TestFleetRuntimePinSurvivesLostDirectoriesAndUnpin(t *testing.T) {
	for _, origin := range []bool{false, true} {
		name := "ordinary"
		if origin {
			name = "origin"
		}
		t.Run(name, func(t *testing.T) {
			backend := newFleetRuntimeBackend(t)
			open := func(identity, pin string) *Runtime {
				session := &fleetTestSession{backend: backend, session: identity + "-process"}
				option := WithFleetStore(session)
				if origin {
					config := originTestConfig()
					config.Bootstrap = true
					option = WithFleetAuthorityOrigin(session, config)
				}
				return openTestRuntime(t, option, WithSchedulerIdentity(identity), WithSource(newStubSource("fleet-source")),
					WithGenerationPin(pin), withScheduleTimer(newStubScheduleTimer().after))
			}
			leader := open("leader", "")
			at := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
			if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", at, false)); err != nil {
				t.Fatal(err)
			}
			selected := leader.State().GenerationID
			if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "second", at.Add(time.Minute), true)); err != nil {
				t.Fatal(err)
			}
			beforePin := leader.State()
			if err := leader.Close(); err != nil {
				t.Fatal(err)
			}
			pinned := open("pin-owner", selected)
			receipt, durable := pinned.PinAcceptance()
			if !durable || receipt.SelectedGenerationID != selected || receipt.AcceptedGenerationID != pinned.State().GenerationID {
				t.Fatal("pin acceptance was not retained with the selected publication")
			}
			if origin && pinned.State().AuthorityHead.Sequence <= beforePin.AuthorityHead.Sequence {
				t.Fatal("origin rollback did not advance the authority sequence")
			}
			follower := open("pin-follower", selected)
			followed, ok := follower.PinAcceptance()
			if !ok || !reflect.DeepEqual(followed, receipt) || follower.State().GenerationID != pinned.State().GenerationID {
				t.Fatal("pin follower did not retain the shared acceptance")
			}
			if err := follower.Close(); err != nil {
				t.Fatal(err)
			}
			directory := pinned.config.stateDirectory
			if err := pinned.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(directory); err != nil {
				t.Fatal(err)
			}
			restarted := open("pin-replacement", selected)
			retained, ok := restarted.PinAcceptance()
			if !ok || !reflect.DeepEqual(retained, receipt) {
				t.Fatalf("pin restart changed the shared acceptance receipt: retained=%+v expected=%+v replay=%v", retained, receipt, restarted.fleetReplayError)
			}
			if _, err := restarted.PublishObservations(t.Context(), manualTestObservation(t, "forbidden", at, false)); err == nil {
				t.Fatal("a pinned runtime accepted acquisition")
			}
			if err := restarted.Close(); err != nil {
				t.Fatal(err)
			}
			unpin := open("unpin-owner", "")
			provider, err := unpin.Catalog().Provider("manual-provider")
			if err != nil || provider.Models["first"] == nil || provider.Models["second"] == nil {
				t.Fatalf("unpin lost acquisition inputs: %v", err)
			}
			if _, present := unpin.PinAcceptance(); present {
				t.Fatal("unpin retained an active acceptance")
			}
			if origin {
				permission, err := unpin.ReadPermission(t.Context())
				if err != nil || permission.Head != unpin.State().AuthorityHead {
					t.Fatalf("origin permission did not observe the current fleet head: %v", err)
				}
			}
		})
	}
}

func TestFleetRuntimeRetainsGrantCapturedBeforeSourceRead(t *testing.T) {
	for _, scenario := range []string{"expired", "released", "takeover", "same-epoch-new-incarnation"} {
		t.Run(scenario, func(t *testing.T) {
			backend := newFleetRuntimeBackend(t)
			source := newStubSource("fleet-source")
			source.release = make(chan struct{})
			generation := aliasGeneration(t, "new-source")
			source.replies = []SourceRead{{Changed: true, Generation: generation, PublishedAt: generation.Manifest.GeneratedAt, Health: HealthOK}}
			connected := openFleetRuntime(t, backend, "producer", privateRuntimeDirectory(t), WithSource(source), WithSourceRefreshMode("manual"))
			before := connected.State()
			original, err := connected.lease.grant(connected.lease.epoch())
			if err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() { _, err := connected.RefreshSource(t.Context()); finished <- err }()
			<-source.entered
			backend.mu.Lock()
			prior := backend.head
			switch scenario {
			case "expired":
				backend.lease.ExpiresAt = time.Now().Add(-time.Second)
			case "released":
				backend.lease.Holder = ""
			case "takeover":
				backend.epoch++
				backend.lease.Epoch = backend.epoch
				backend.lease.Holder = "other"
				backend.lease.SessionID = "other-process"
			case "same-epoch-new-incarnation":
				backend.identity.RecoveryEpoch++
				backend.identity.BackendID = "replacement-process"
				backend.lease.Identity = backend.identity
				backend.lease.Epoch = original.Epoch
			}
			replacement := backend.lease
			backend.mu.Unlock()
			if scenario == "same-epoch-new-incarnation" {
				connected.lease.mu.Lock()
				connected.lease.lease = replacement
				connected.lease.mu.Unlock()
			}
			close(source.release)
			if err := <-finished; err == nil {
				t.Fatal("an obsolete acquisition grant published a catalog")
			}
			backend.mu.Lock()
			head := backend.head
			backend.mu.Unlock()
			if head != prior || connected.State().GenerationID != before.GenerationID {
				t.Fatal("refused publication changed the selected catalog or recovery reference")
			}
		})
	}
}

func TestFleetRuntimeAuthorityRecoveryDoesNotRecoverPermission(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	backend.head = FleetHead{}
	backend.snapshots = make(map[FleetHead]FleetSnapshot)
	open := func(identity string) (*Runtime, *authorityTestSource) {
		source, options := authorityRuntimeFixture(t)
		session := &fleetTestSession{backend: backend, session: identity + "-process"}
		options = append(options, WithFleetStore(session), WithSchedulerIdentity(identity),
			withScheduleTimer(newStubScheduleTimer().after))
		return openTestRuntime(t, options...), source
	}
	leader, _ := open("authority-leader")
	if leader.AllowsNewAttempt() {
		t.Fatal("an empty shared store authorized the embedded catalog")
	}
	if _, err := leader.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !leader.AllowsNewAttempt() {
		t.Fatal("the leader did not activate the acquired authority permission")
	}
	follower, source := open("authority-follower")
	if follower.State().AuthorityHead != leader.State().AuthorityHead {
		t.Fatal("the follower lost the exact upstream authority head")
	}
	status, _ := follower.FleetStatus()
	if !status.ReplayReady || follower.AllowsNewAttempt() {
		t.Fatal("shared catalog recovery either failed or invented permission")
	}
	if err := follower.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !follower.AllowsNewAttempt() {
		t.Fatal("an independently validated receipt did not authorize the follower")
	}
	source.permissionMu.Lock()
	source.receipt.Head.Sequence++
	source.receipt.Head.GenerationID = "withdrawn-generation"
	source.receipt.Head.RequiredPermissionRevision = "sha256:" + fleetRecoveryChecksum([]byte("withdrawn-revision"))
	source.permissionMu.Unlock()
	if err := follower.RefreshPermission(t.Context()); err != nil {
		t.Fatal(err)
	}
	if follower.AllowsNewAttempt() {
		t.Fatal("a known withdrawal retained permission for the previous fleet generation")
	}
	if err := follower.RefreshFleet(t.Context()); err != nil {
		t.Fatal(err)
	}
	if follower.AllowsNewAttempt() {
		t.Fatal("rereading the shared catalog restored withdrawn permission")
	}
}

func TestFleetRuntimeRejectsRegressedOrReplacedHead(t *testing.T) {
	for _, scenario := range []string{"missing", "regressed", "same-revision", "recovery-identity"} {
		t.Run(scenario, func(t *testing.T) {
			backend := newFleetRuntimeBackend(t)
			leader := openFleetRuntime(t, backend, "leader", privateRuntimeDirectory(t))
			if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", time.Now().UTC(), false)); err != nil {
				t.Fatal(err)
			}
			follower := openFleetRuntime(t, backend, "follower", privateRuntimeDirectory(t))
			before := follower.State()
			backend.mu.Lock()
			switch scenario {
			case "missing":
				backend.head = FleetHead{}
			case "regressed":
				backend.head.Revision--
			case "same-revision":
				backend.head.GenerationID = "different-generation"
			case "recovery-identity":
				backend.head.Identity.BackendID = "replacement-backend"
			}
			backend.mu.Unlock()
			if err := follower.RefreshFleet(t.Context()); err == nil {
				t.Fatal("follower accepted a regressed or replaced publication head")
			}
			if after := follower.State(); after.GenerationID != before.GenerationID || after.PayloadChecksum != before.PayloadChecksum {
				t.Fatal("refused head replaced the serving catalog")
			}
		})
	}
}

func TestFleetRuntimeRefusesUnacceptedPinOnFollower(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	leader := openFleetRuntime(t, backend, "leader", privateRuntimeDirectory(t))
	selected := leader.State().GenerationID
	if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", time.Now().UTC(), false)); err != nil {
		t.Fatal(err)
	}
	head := backend.head
	session := &fleetTestSession{backend: backend, session: "pin-follower-process"}
	follower, err := Open(t.Context(), WithFleetStore(session), WithSchedulerIdentity("pin-follower"),
		WithStateDirectory(privateRuntimeDirectory(t)), WithSource(newStubSource("fleet-source")),
		WithGenerationPin(selected), withScheduleTimer(newStubScheduleTimer().after))
	if follower != nil {
		_ = follower.Close()
	}
	if !errors.IsConflict(err) || follower != nil {
		t.Fatalf("follower served an unaccepted pin: %v", err)
	}
	if backend.head != head {
		t.Fatal("refused follower changed the shared head")
	}
}
