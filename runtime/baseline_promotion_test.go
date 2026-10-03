package runtime

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// newEmbeddedFleetBackend publishes the retained "baseline" fixture without a source layer.
// The packaged baseline of the test binary differs from it.
func newEmbeddedFleetBackend(t *testing.T) *fleetTestBackend {
	t.Helper()
	layers, generation := embeddedFleetLayers(t)
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	return fleetBackendWithRecovery(t, generation, data)
}

func embeddedFleetLayers(t *testing.T) (layerSet, catalogs.Generation) {
	t.Helper()
	layers := fleetTestLayers(t)
	generation := aliasGeneration(t, "baseline")
	layers.fleetBaseline = &generation
	var err error
	layers.sourceConfiguration, err = describeSources(defaults())
	if err != nil {
		t.Fatal(err)
	}
	state, err := layers.build(t.Context(), layers.embedded)
	if err != nil {
		t.Fatal(err)
	}
	if state.GenerationID != generation.Manifest.GenerationID || state.PayloadChecksum != generation.Manifest.Payload.Checksum {
		t.Fatal("the embedded-only fixture does not reproduce its baseline")
	}
	return layers, generation
}

func fleetBackendWithRecovery(t *testing.T, generation catalogs.Generation, data []byte) *fleetTestBackend {
	t.Helper()
	p := fleetTestPublication(t)
	p.Generation = generation
	p.Recovery = FleetRecovery{GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
		Checksum: fleetRecoveryChecksum(data), Data: data}
	snapshot := FleetSnapshot{Head: p.nextHead(), Publication: p}
	return &fleetTestBackend{identity: p.Grant.Identity, head: snapshot.Head, epoch: p.Grant.Epoch,
		snapshots: map[FleetHead]FleetSnapshot{snapshot.Head: snapshot}, generations: map[string]catalogs.Generation{generation.Manifest.GenerationID: generation}}
}

func openEmbeddedFleetRuntime(t *testing.T, b *fleetTestBackend, identity string, options ...Option) *Runtime {
	t.Helper()
	session := &fleetTestSession{backend: b, session: identity + "-process"}
	selected := []Option{WithFleetStore(session), WithSchedulerIdentity(identity), withScheduleTimer(newStubScheduleTimer().after)}
	return openTestRuntime(t, append(selected, options...)...)
}

func (b *fleetTestBackend) currentRecord(t *testing.T) (FleetSnapshot, fleetRecoveryRecord) {
	t.Helper()
	b.mu.Lock()
	snapshot := b.snapshots[b.head]
	b.mu.Unlock()
	record, err := readFleetRecovery(t.Context(), snapshot.Publication.Recovery.Data)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, record
}

func (b *fleetTestBackend) currentHead() FleetHead {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.head
}

func packagedIdentity(r *Runtime) catalogs.GenerationIdentity {
	state := r.Client().EmbeddedCatalogState()
	return catalogs.GenerationIdentity{GenerationID: state.GenerationID, PayloadChecksum: state.PayloadChecksum}
}

func promoteStatus(t *testing.T, r *Runtime) (BaselinePromotionResult, error) {
	t.Helper()
	status, ok := r.BaselineStatus()
	if !ok {
		t.Fatal("fleet runtime reported no baseline status")
	}
	return r.PromoteEmbeddedBaseline(t.Context(), BaselinePromotion{ExpectedHead: status.Head, PackagedGenerationID: status.Packaged.GenerationID})
}

func TestPromoteEmbeddedBaselineAdoptsPackagedBaselineUnderLease(t *testing.T) {
	backend := newEmbeddedFleetBackend(t)
	leader := openEmbeddedFleetRuntime(t, backend, "leader")
	follower := openEmbeddedFleetRuntime(t, backend, "follower")
	_, before := backend.currentRecord(t)
	status, ok := leader.BaselineStatus()
	packaged := packagedIdentity(leader)
	retained := catalogs.GenerationIdentity{GenerationID: "baseline", PayloadChecksum: leader.layers.embedded.PayloadChecksum}
	if !ok || !status.Promotable || status.Refusal != "" || status.Packaged != packaged || status.Retained != retained || status.Head != backend.currentHead() {
		t.Fatalf("baseline status: %+v", status)
	}
	request := BaselinePromotion{ExpectedHead: status.Head, PackagedGenerationID: packaged.GenerationID}
	if _, err := follower.PromoteEmbeddedBaseline(t.Context(), request); err == nil {
		t.Fatal("a replica without the publication lease promoted the baseline")
	}
	if backend.currentHead() != status.Head {
		t.Fatal("a refused promotion changed the shared head")
	}
	result, err := leader.PromoteEmbeddedBaseline(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	head := backend.currentHead()
	if result.Head != head || head.Revision <= status.Head.Revision || head.Identity != status.Head.Identity {
		t.Fatalf("promotion head %+v after %+v, store %+v", result.Head, status.Head, head)
	}
	if result.Previous != retained || result.Promoted != packaged || len(result.InertRemovals) != 0 {
		t.Fatalf("promotion receipt: %+v", result)
	}
	if leader.layers.embedded.GenerationID != packaged.GenerationID || leader.State().GenerationID != packaged.GenerationID {
		t.Fatal("the leader did not serve the promoted embedded-only catalog")
	}
	snapshot, record := backend.currentRecord(t)
	backend.mu.Lock()
	grant := backend.lease
	backend.mu.Unlock()
	if !sameLeaseGrant(snapshot.Publication.Grant, grant) || grant.Holder != "leader" {
		t.Fatal("promotion was not published under the leader's grant")
	}
	if record.Version != fleetRecoveryVersion || record.Baseline.Manifest.GenerationID != packaged.GenerationID || record.Baseline.Manifest.Payload.Checksum != packaged.PayloadChecksum {
		t.Fatal("the recovery record does not retain the promoted baseline")
	}
	if record.Compatibility == before.Compatibility {
		t.Fatal("promotion kept the previous compatibility value")
	}
	after, _ := leader.BaselineStatus()
	if after.Promotable || after.Retained != packaged || after.Head != head {
		t.Fatalf("status after promotion: %+v", after)
	}
}

func TestFleetReplayRestoresPromotedBaseline(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	leader := openFleetRuntime(t, backend, "leader", privateRuntimeDirectory(t))
	follower := openFleetRuntime(t, backend, "follower", privateRuntimeDirectory(t))
	at := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", at, false)); err != nil {
		t.Fatal(err)
	}
	served := leader.State()
	result, err := promoteStatus(t, leader)
	if err != nil {
		t.Fatal(err)
	}
	packaged := packagedIdentity(leader)
	if result.Promoted != packaged || leader.State().GenerationID != served.GenerationID || leader.State().PayloadChecksum != served.PayloadChecksum {
		t.Fatal("promotion changed the catalog that the retained source selects")
	}
	if err := follower.RefreshFleet(t.Context()); err != nil {
		t.Fatal(err)
	}
	replica := openFleetRuntime(t, backend, "replica", privateRuntimeDirectory(t))
	for name, r := range map[string]*Runtime{"follower": follower, "replica": replica} {
		status, _ := r.FleetStatus()
		if !status.ReplayReady || status.Head != result.Head {
			t.Fatalf("%s did not replay the promoted head: %+v", name, status)
		}
		if r.layers.embedded.GenerationID != packaged.GenerationID || r.layers.embedded.PayloadChecksum != packaged.PayloadChecksum ||
			r.layers.fleetBaseline == nil || r.layers.fleetBaseline.Manifest.GenerationID != packaged.GenerationID {
			t.Fatalf("%s did not restore the promoted baseline", name)
		}
		if r.layers.source == nil || r.layers.source.Identity != "fleet-source" || r.layers.manual == nil {
			t.Fatalf("%s lost retained source or manual inputs", name)
		}
		if r.State().GenerationID != served.GenerationID || r.State().PayloadChecksum != served.PayloadChecksum {
			t.Fatalf("%s serves a different catalog", name)
		}
	}
}

func TestPromoteEmbeddedBaselineRefusesStaleHeadAndEqualBaseline(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	leader := openFleetRuntime(t, backend, "leader", privateRuntimeDirectory(t))
	stale, _ := leader.BaselineStatus()
	if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), false)); err != nil {
		t.Fatal(err)
	}
	current := backend.currentHead()
	packaged := packagedIdentity(leader)
	for _, scenario := range []struct {
		name     string
		request  BaselinePromotion
		conflict bool
	}{
		{"stale-head", BaselinePromotion{ExpectedHead: stale.Head, PackagedGenerationID: packaged.GenerationID}, true},
		{"other-packaged-generation", BaselinePromotion{ExpectedHead: current, PackagedGenerationID: "older-binary"}, true},
		{"missing-head", BaselinePromotion{PackagedGenerationID: packaged.GenerationID}, false},
		{"missing-packaged-generation", BaselinePromotion{ExpectedHead: current}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			_, err := leader.PromoteEmbeddedBaseline(t.Context(), scenario.request)
			if scenario.conflict && !errors.IsConflict(err) || !scenario.conflict && !errors.IsValidationError(err) {
				t.Fatalf("refusal: %v", err)
			}
			if backend.currentHead() != current || leader.layers.embedded.GenerationID != "baseline" {
				t.Fatal("a refused promotion changed the shared head or retained baseline")
			}
		})
	}
	promoted, err := promoteStatus(t, leader)
	if err != nil {
		t.Fatal(err)
	}
	status, _ := leader.BaselineStatus()
	if status.Promotable || status.Refusal == "" || status.Packaged != status.Retained {
		t.Fatalf("status after promotion: %+v", status)
	}
	_, err = leader.PromoteEmbeddedBaseline(t.Context(), BaselinePromotion{ExpectedHead: promoted.Head, PackagedGenerationID: packaged.GenerationID})
	if !errors.IsConflict(err) || !strings.Contains(err.Error(), "retained") {
		t.Fatalf("equal baseline promotion: %v", err)
	}
	if backend.currentHead() != promoted.Head {
		t.Fatal("an equal baseline promotion published again")
	}
	local := openTestRuntime(t)
	if _, ok := local.BaselineStatus(); ok {
		t.Fatal("a local runtime reported fleet baseline status")
	}
	if _, err := local.PromoteEmbeddedBaseline(t.Context(), BaselinePromotion{ExpectedHead: current, PackagedGenerationID: packaged.GenerationID}); !errors.IsConflict(err) {
		t.Fatalf("local promotion: %v", err)
	}
}

func TestPromoteEmbeddedBaselineRefusesPinnedGeneration(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	leader := openFleetRuntime(t, backend, "leader", privateRuntimeDirectory(t))
	selected := leader.State().GenerationID
	if _, err := leader.PublishObservations(t.Context(), manualTestObservation(t, "first", time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), false)); err != nil {
		t.Fatal(err)
	}
	if err := leader.Close(); err != nil {
		t.Fatal(err)
	}
	pinned := openFleetRuntime(t, backend, "pin-owner", privateRuntimeDirectory(t), WithGenerationPin(selected))
	if _, durable := pinned.PinAcceptance(); !durable {
		t.Fatal("test requires an accepted shared pin")
	}
	head := backend.currentHead()
	status, ok := pinned.BaselineStatus()
	if !ok || status.Promotable || !strings.Contains(status.Refusal, "pin") || status.Head != head {
		t.Fatalf("pinned status: %+v", status)
	}
	_, err := pinned.PromoteEmbeddedBaseline(t.Context(), BaselinePromotion{ExpectedHead: head, PackagedGenerationID: status.Packaged.GenerationID})
	if !errors.IsConflict(err) || !strings.Contains(err.Error(), "pin") {
		t.Fatalf("pinned promotion: %v", err)
	}
	if backend.currentHead() != head || pinned.layers.embedded.GenerationID != "baseline" {
		t.Fatal("a pinned promotion changed the shared head or retained baseline")
	}
}

func TestPromoteEmbeddedBaselineRefusesConfiguredAuthority(t *testing.T) {
	backend := newFleetRuntimeBackend(t)
	session := &fleetTestSession{backend: backend, session: "origin-process"}
	config := originTestConfig()
	config.Bootstrap = true
	origin := openTestRuntime(t, WithFleetAuthorityOrigin(session, config), WithSchedulerIdentity("origin"),
		WithSource(newStubSource("fleet-source")), withScheduleTimer(newStubScheduleTimer().after))
	head := backend.currentHead()
	status, ok := origin.BaselineStatus()
	if !ok || status.Promotable || !strings.Contains(status.Refusal, "authority") {
		t.Fatalf("origin status: %+v", status)
	}
	_, err := origin.PromoteEmbeddedBaseline(t.Context(), BaselinePromotion{ExpectedHead: status.Head, PackagedGenerationID: status.Packaged.GenerationID})
	if !errors.IsConflict(err) || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("origin promotion: %v", err)
	}
	if backend.currentHead() != head {
		t.Fatal("an origin promotion changed the shared head")
	}

	source, options := authorityRuntimeFixture(t)
	required := openFleetRuntime(t, newFleetRuntimeBackend(t), "authority-consumer", privateRuntimeDirectory(t), append(options, WithSource(source))...)
	if !required.requiresAuthority() {
		t.Fatal("test requires a source authority")
	}
	status, _ = required.BaselineStatus()
	if status.Promotable || !strings.Contains(status.Refusal, "authority") {
		t.Fatalf("authority status: %+v", status)
	}
	_, err = required.PromoteEmbeddedBaseline(t.Context(), BaselinePromotion{ExpectedHead: status.Head, PackagedGenerationID: status.Packaged.GenerationID})
	if !errors.IsConflict(err) || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("authority promotion: %v", err)
	}
}

func TestPromoteEmbeddedBaselineRetainsInertRemovalTargets(t *testing.T) {
	backend := newEmbeddedFleetBackend(t)
	leader := openEmbeddedFleetRuntime(t, backend, "leader")
	target, err := catalogs.NewCanonicalRemovalTarget("author/current")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leader.ReplaceRemovalTargets(t.Context(), leader.State(), target); err != nil {
		t.Fatal(err)
	}
	if _, err := leader.Client().EmbeddedCatalogState().Catalog.Definition("author/current"); err == nil {
		t.Fatal("test requires a target absent from the packaged baseline")
	}
	result, err := promoteStatus(t, leader)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.InertRemovals, []catalogs.CatalogRemovalTarget{target}) {
		t.Fatalf("receipt inert removals: %+v", result.InertRemovals)
	}
	_, record := backend.currentRecord(t)
	if record.Removals == nil || !reflect.DeepEqual(record.Removals.Targets, []catalogs.CatalogRemovalTarget{target}) {
		t.Fatal("promotion dropped the retained removal target")
	}
	replica := openEmbeddedFleetRuntime(t, backend, "replica")
	for name, r := range map[string]*Runtime{"leader": leader, "replica": replica} {
		if r.layers.removals == nil || !reflect.DeepEqual(r.layers.removals.Targets, []catalogs.CatalogRemovalTarget{target}) {
			t.Fatalf("%s lost the retained removal target", name)
		}
		if !r.Catalog().Removals().ContainsCanonical("author/current") {
			t.Fatalf("%s does not publish the retained removal policy", name)
		}
		if r.layers.embedded.GenerationID != result.Promoted.GenerationID {
			t.Fatalf("%s did not adopt the promoted baseline", name)
		}
	}
	if _, err := leader.ReplaceRemovalTargets(t.Context(), leader.State()); err != nil {
		t.Fatal("explicit restore of an inert target:", err)
	}
	if leader.layers.removals == nil || len(leader.layers.removals.Targets) != 0 {
		t.Fatal("explicit restore kept the inert target")
	}
}

func TestFleetReplayAfterPromotionIgnoresOlderPackagedBaseline(t *testing.T) {
	backend := newEmbeddedFleetBackend(t)
	leader := openEmbeddedFleetRuntime(t, backend, "leader")
	result, err := promoteStatus(t, leader)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, record := backend.currentRecord(t)
	if record.Version != fleetRecoveryVersion {
		t.Fatalf("promotion changed the recovery record version to %d", record.Version)
	}
	older := fleetTestLayers(t)
	generation := aliasGeneration(t, "baseline")
	rollback := leader.layers
	rollback.embedded, rollback.embeddedManifest, rollback.fleetBaseline = older.embedded, older.embeddedManifest, &generation
	recovered, _, err := recoverFleetState(t.Context(), snapshot, rollback)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.embedded.GenerationID != result.Promoted.GenerationID || recovered.embedded.PayloadChecksum != result.Promoted.PayloadChecksum {
		t.Fatal("the older binary fell back to its packaged baseline")
	}
	state, err := recovered.build(t.Context(), recovered.embedded)
	if err != nil {
		t.Fatal(err)
	}
	if state.PayloadChecksum != snapshot.Publication.Generation.Manifest.Payload.Checksum {
		t.Fatal("the older binary did not reproduce the promoted publication")
	}
	if _, _, err := recoverFleetState(t.Context(), snapshot, older); err == nil {
		t.Fatal("replay accepted a binary without the publication's acquisition policy")
	}
}

func TestFleetRecoveryRefusesUnsupportedRecordVersion(t *testing.T) {
	layers, generation := embeddedFleetLayers(t)
	raw, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	record, err := readFleetRecovery(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	unsupported := fleetRecoveryVersion + 1
	record.Version = unsupported
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	data := fleetCompressedTest(t, encoded)
	extended := fleetCompressedTest(t, append([]byte(`{"future_member":true,`), encoded[1:]...))
	for name, raw := range map[string][]byte{"same-members": data, "new-member": extended} {
		if _, err := decodeFleetRecovery(t.Context(), raw); !errors.IsConflict(err) || !strings.Contains(err.Error(), "version "+strconv.Itoa(unsupported)) {
			t.Fatalf("%s: unsupported version: %v", name, err)
		}
	}
	backend := fleetBackendWithRecovery(t, generation, data)
	replica := openEmbeddedFleetRuntime(t, backend, "newer-format")
	status, ok := replica.FleetStatus()
	if !ok || status.ReplayReady || status.AcquisitionReady || replica.lease.status() != leaseLost {
		t.Fatalf("unsupported record status: %+v", status)
	}
	if replica.fleetReplayError == nil || !strings.Contains(replica.fleetReplayError.Error(), "version "+strconv.Itoa(unsupported)) {
		t.Fatalf("replay error does not name the version: %v", replica.fleetReplayError)
	}
	if replica.State().GenerationID != generation.Manifest.GenerationID {
		t.Fatal("an unsupported record hid the accepted catalog")
	}
	if _, err := replica.PublishObservations(t.Context(), manualTestObservation(t, "forbidden", time.Now().UTC(), false)); err == nil {
		t.Fatal("a replica with an unsupported record acquired refresh ownership")
	}
	backend.mu.Lock()
	acquisitions := backend.acquisitions
	backend.mu.Unlock()
	if acquisitions != 0 {
		t.Fatal("a replica with an unsupported record tried to acquire the lease")
	}
}
