package runtime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func materializationFixture(t *testing.T) (CatalogMaterializationRequest, []Option, storage.Store) {
	t.Helper()
	snapshot, layers := fleetValidationFixture(t)
	opts := []Option{WithCatalogSource("embedded"), WithCatalogNetworkMode("offline"), WithAcquisitionEnabled(false), WithRetentionEnabled(false)}
	config := defaults()
	if _, err := config.apply(opts...); err != nil {
		t.Fatal(err)
	}
	config.resolve()
	var err error
	layers.sourceConfiguration, err = describeSources(config)
	if err != nil {
		t.Fatal(err)
	}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data = data
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(data)
	snapshot.Head = snapshot.Publication.nextHead()
	recovery, err := CaptureFleetCatalogRecovery(t.Context(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	directory := privateRuntimeDirectory(t)
	if err := bindDirectoryOwner(t.Context(), directory, config.directoryOwner, config.schedulerIdentity); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareInstanceSeed(t.Context(), directory); err != nil {
		t.Fatal(err)
	}
	if _, err := newLayerStore(directory); err != nil {
		t.Fatal(err)
	}
	store := storage.NewMemory()
	if err := store.Commit(t.Context(), snapshot.Publication.Generation, ""); err != nil {
		t.Fatal(err)
	}
	return CatalogMaterializationRequest{Directory: directory, Owner: config.directoryOwner, SchedulerIdentity: config.schedulerIdentity, OperationID: "materialize-test", Inputs: []CatalogMaterializationInput{{Generation: snapshot.Publication.Generation, Recovery: recovery}}}, opts, store
}

func TestCatalogMaterializationRetainsExplicitHistoricalBaseline(t *testing.T) {
	request, opts, store := materializationFixture(t)
	before, err := os.ReadFile(filepath.Join(request.Directory, instanceSeedFileName))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SelectedInputsSHA256 != request.Inputs[0].Recovery.Inputs.Checksum || receipt.SelectedManifestSHA256 != request.Inputs[0].Recovery.ManifestChecksum || receipt.PlanSHA256 == "" {
		t.Fatal("missing exact completion bindings")
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
	original := request.Inputs[0].Generation
	r := openTestRuntime(t, append(opts, WithStateDirectory(request.Directory), WithClientOptions(starmap.WithCatalogStore(store)))...)
	if r.Status().RecoveryBaselineSHA256 != receipt.BaselineManifestSHA256 || r.layers.embedded.GenerationID != original.Manifest.GenerationID || r.layers.embedded.PayloadChecksum != original.Manifest.Payload.Checksum {
		t.Fatal("binary replaced the explicit recovered baseline")
	}
	if r.State().PayloadChecksum != original.Manifest.Payload.Checksum {
		t.Fatal("restart changed accepted payload")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	// Ordinary publication must retain the recovery baseline in its semantic inputs.
	layers := r.layers
	observations, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "later", original.Manifest.GeneratedAt.Add(time.Minute), false)})
	if err != nil {
		t.Fatal(err)
	}
	layers.manual = &manualBatch{observations: observations}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	record, err := readFleetRecovery(t.Context(), data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(record.Baseline.Payload, original.Payload) {
		t.Fatal("later commit captured the binary baseline")
	}
	files, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := json.Marshal(inputPublication{Version: inputPublicationVersion, Phase: inputPublicationIdle}, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	changed = append(changed, '\n')
	if err := files.directory.PublishFileContext(t.Context(), inputPublicationName, changed, ".layer-"); err != nil {
		t.Fatal(err)
	}
	retry, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil || retry != receipt {
		t.Fatalf("exact completed retry: %v %v", retry, err)
	}
	current, err := files.directory.ReadFile(inputPublicationName, maxLayerBytes)
	if err != nil || !bytes.Equal(current, changed) {
		t.Fatal("retry replaced later state", err)
	}
	seed, err := os.ReadFile(filepath.Join(request.Directory, instanceSeedFileName))
	if err != nil || !bytes.Equal(seed, before) {
		t.Fatal("materialization changed replica identity", err)
	}
}

func TestCatalogMaterializationPreservesHistoricalInventory(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	older := request.Inputs[0]
	next := older.Generation.Copy()
	next.Manifest.GenerationID += "-candidate"
	recovery := older.Recovery
	manifest, err := catalogManifestChecksum(next)
	if err != nil {
		t.Fatal(err)
	}
	recovery.ManifestChecksum = manifest
	recovery.Inputs.GenerationID = next.Manifest.GenerationID
	request.Inputs = append(request.Inputs, CatalogMaterializationInput{Generation: next, Recovery: recovery})
	request.Selected = 1
	receipt, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SelectedManifestSHA256 != manifest {
		t.Fatal("accepted generation replaced selected candidate")
	}
	for _, input := range request.Inputs {
		checksums, err := CatalogRecoveryChecksums(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity, input.Generation)
		if err != nil || len(checksums) != 1 {
			t.Fatal("historical inputs missing", checksums, err)
		}
		captured, err := ReadCatalogRecovery(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity, input.Generation, checksums[0])
		if err != nil || !reflect.DeepEqual(captured, input.Recovery) {
			t.Fatal("historical inputs changed", err)
		}
	}
	request.OperationID = "materialize-second"
	request.Selected = 0
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal("later explicit operation", err)
	}
}

type cutMaterializationContext struct {
	context.Context
	marker string
}

func (c cutMaterializationContext) Err() error {
	if _, err := os.Stat(c.marker); err == nil {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestCatalogMaterializationInterruptionRequiresExactRecovery(t *testing.T) {
	for _, point := range []string{materializationSelectionName, recoveryBaselineName} {
		t.Run(point, func(t *testing.T) {
			request, opts, _ := materializationFixture(t)
			ctx := cutMaterializationContext{Context: t.Context(), marker: filepath.Join(request.Directory, layerDirectoryName, point)}
			if _, err := MaterializeCatalogRecovery(ctx, request, opts...); err == nil {
				t.Fatal("interruption did not stop materialization")
			}
			if _, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory))...); err == nil {
				t.Fatal("pending materialization allowed runtime startup")
			}
			changed := request
			changed.Selected = 0
			changed.Inputs = slicesCloneMaterializationInputs(request.Inputs)
			changed.Inputs[0].Generation.Manifest.GenerationID += "-changed"
			if _, err := MaterializeCatalogRecovery(t.Context(), changed, opts...); err == nil {
				t.Fatal("changed operation inputs accepted")
			}
			if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
				t.Fatal("exact interrupted retry", err)
			}
		})
	}
}

func slicesCloneMaterializationInputs(input []CatalogMaterializationInput) []CatalogMaterializationInput {
	result := append([]CatalogMaterializationInput(nil), input...)
	for i := range result {
		result[i].Generation = result[i].Generation.Copy()
	}
	return result
}

func TestCatalogMaterializationRefusesConcurrentOwnerAndChangedState(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	lock, err := acquireDirectory(t.Context(), request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("active runtime owner accepted")
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := cutMaterializationContext{Context: t.Context(), marker: filepath.Join(request.Directory, layerDirectoryName, materializationSelectionName)}
	if _, err := MaterializeCatalogRecovery(ctx, request, opts...); err == nil {
		t.Fatal("interruption did not occur")
	}
	files, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := files.directory.PublishFileContext(t.Context(), recoveryBaselineName, []byte("changed"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("changed target inputs were overwritten")
	}
	data, err := files.directory.ReadFile(recoveryBaselineName, maxLayerBytes)
	if err != nil || string(data) != "changed" {
		t.Fatal("changed input lost", err)
	}
}

func TestCatalogMaterializationFullEmbeddedContract(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	baseline, err := bootstrap.Generation()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeCatalogGeneration(baseline)
	if err != nil {
		t.Fatal(err)
	}
	config := defaults()
	if _, err := config.apply(opts...); err != nil {
		t.Fatal(err)
	}
	config.resolve()
	description, err := describeSources(config)
	if err != nil {
		t.Fatal(err)
	}
	layers := layerSet{publisherID: "deployment", embeddedManifest: &baseline.Manifest, embedded: starmap.CatalogState{Catalog: catalog, GenerationID: baseline.Manifest.GenerationID, PayloadChecksum: baseline.Manifest.Payload.Checksum, GeneratedAt: baseline.Manifest.GeneratedAt}, sourceConfiguration: description}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := catalogManifestChecksum(baseline)
	if err != nil {
		t.Fatal(err)
	}
	request.Inputs = []CatalogMaterializationInput{{Generation: baseline, Recovery: CatalogRecovery{ManifestChecksum: manifest, Inputs: FleetRecovery{GenerationID: baseline.Manifest.GenerationID, PayloadChecksum: baseline.Manifest.Payload.Checksum, Checksum: fleetRecoveryChecksum(data), Data: data}}}}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogMaterializationRejectsUnsafeRequest(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	for _, mutate := range []func(*CatalogMaterializationRequest){
		func(r *CatalogMaterializationRequest) { r.Directory = "relative" },
		func(r *CatalogMaterializationRequest) { r.OperationID = strings.Repeat("x", deploymentIDMaxBytes+1) },
		func(r *CatalogMaterializationRequest) { r.OperationID = "bad\noperation" },
		func(r *CatalogMaterializationRequest) { r.Selected = 1 },
		func(r *CatalogMaterializationRequest) {
			r.Inputs = make([]CatalogMaterializationInput, storage.DefaultRetentionScanEntries+1)
		},
		func(r *CatalogMaterializationRequest) { r.Inputs = append(r.Inputs, r.Inputs[0]) },
		func(r *CatalogMaterializationRequest) { r.Owner.Deployment = "foreign" },
	} {
		changed := request
		mutate(&changed)
		if _, err := MaterializeCatalogRecovery(t.Context(), changed, opts...); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if _, err := MaterializeCatalogRecovery(nil, request, opts...); err == nil {
		t.Fatal("nil context accepted")
	}
	parent, err := privatefiles.ExistingDirectory(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.PublishFileContext(t.Context(), instanceSeedFileName, []byte("changed"), ".owner-"); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("changed identity accepted")
	}
}

func TestCatalogMaterializationPreservesUncertainPermission(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	permission := authorityPermissionFixture()
	p, err := (authorityPermissions{authorityID: permission.Head.AuthorityID, policyID: permission.Head.PolicyID}).observe(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.savePermission(t.Context(), p, true); err != nil {
		t.Fatal(err)
	}
	before, err := store.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal(err)
	}
	after, err := store.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("catalog materialization altered permission", err)
	}
	restored, err := store.loadPermission(authorityPermissions{authorityID: permission.Head.AuthorityID, policyID: permission.Head.PolicyID})
	if err != nil || restored.retained || restored.pending {
		t.Fatal("catalog materialization granted uncertain permission", err)
	}
	request.OperationID = "interrupted-permission"
	ctx := cutMaterializationContext{Context: t.Context(), marker: filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationPlanName)}
	// This cut stops after the writer prepares the journal and before catalog replacement.
	if _, err := MaterializeCatalogRecovery(ctx, request, opts...); err == nil {
		t.Fatal("missing interruption")
	}
	if err := store.savePermission(t.Context(), p, false); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("changed permission continuity accepted")
	}
}

type exitMaterializationContext struct {
	context.Context
	marker string
}

func (c exitMaterializationContext) Err() error {
	if _, err := os.Stat(c.marker); err == nil {
		os.Exit(88)
	}
	return c.Context.Err()
}

func TestCatalogMaterializationRecoversProcessExit(t *testing.T) {
	if requestPath := os.Getenv("STARMAP_TEST_MATERIALIZATION_REQUEST"); requestPath != "" {
		data, err := os.ReadFile(requestPath)
		if err != nil {
			t.Fatal(err)
		}
		var request CatalogMaterializationRequest
		if err := json.Unmarshal(data, &request); err != nil {
			t.Fatal(err)
		}
		ctx := exitMaterializationContext{Context: t.Context(), marker: os.Getenv("STARMAP_TEST_MATERIALIZATION_MARKER")}
		_, err = MaterializeCatalogRecovery(ctx, request, WithCatalogSource("embedded"), WithCatalogNetworkMode("offline"), WithAcquisitionEnabled(false), WithRetentionEnabled(false))
		t.Fatalf("process did not exit after its publication: %v", err)
	}
	for _, point := range []string{"plan", materializationSelectionName, recoveryBaselineName} {
		t.Run(point, func(t *testing.T) {
			request, opts, _ := materializationFixture(t)
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			requestPath := filepath.Join(t.TempDir(), "request.json")
			if err := os.WriteFile(requestPath, data, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(request.Directory, layerDirectoryName, point)
			if point == "plan" {
				marker = filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationPlanName)
			}
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCatalogMaterializationRecoversProcessExit$")
			command.Env = append(os.Environ(), "STARMAP_TEST_MATERIALIZATION_REQUEST="+requestPath, "STARMAP_TEST_MATERIALIZATION_MARKER="+marker)
			output, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v %s", err, output)
			}
			if _, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory))...); err == nil {
				t.Fatal("abandoned materialization allowed startup")
			}
			if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
				t.Fatal("interrupted private publication recovery", err)
			}
			if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogMaterializationRestoresLayersRemovalsAndPinWithoutAuthority(t *testing.T) {
	request, opts, store := materializationFixture(t)
	record, err := readFleetRecovery(t.Context(), request.Inputs[0].Recovery.Inputs.Data)
	if err != nil {
		t.Fatal(err)
	}
	layers, err := decodeFleetRecoveryRecord(t.Context(), record, layerSet{})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "retained-manual", layers.embedded.GeneratedAt.Add(time.Minute), false)})
	if err != nil {
		t.Fatal(err)
	}
	layers.manual = &manualBatch{observations: prepared}
	target, err := catalogs.NewCanonicalRemovalTarget("author/current")
	if err != nil {
		t.Fatal(err)
	}
	layers.removals = &catalogs.CatalogRemovalPolicy{PublisherID: layers.publisherID, Targets: []catalogs.CatalogRemovalTarget{target}}
	state, err := layers.build(t.Context(), layers.embedded)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := starmap.NewCandidate(state.Catalog, layers.buildEvidence, starmap.WithCandidateGenerationID(state.GenerationID))
	if err != nil {
		t.Fatal(err)
	}
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	generation, err := client.PrepareGeneration(t.Context(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	config := defaults()
	if _, err := config.apply(opts...); err != nil {
		t.Fatal(err)
	}
	config.resolve()
	layers.sourceConfiguration, err = describeSources(config)
	if err != nil {
		t.Fatal(err)
	}
	probe := &Runtime{config: *config}
	pin := generationPinRecord{Version: generationPinRecordVersion, Phase: pinAccepted, Binding: probe.pinBinding(), Receipt: GenerationPinAcceptance{OperationID: "original-pin", SelectedGenerationID: generation.Manifest.GenerationID, AcceptedGenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum, RequestedAt: layers.embedded.GeneratedAt.Add(time.Minute), AcceptedAt: layers.embedded.GeneratedAt.Add(2 * time.Minute)}}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, &pin)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := catalogManifestChecksum(generation)
	if err != nil {
		t.Fatal(err)
	}
	request.Inputs = append(request.Inputs, CatalogMaterializationInput{Generation: generation, Recovery: CatalogRecovery{ManifestChecksum: manifest, Inputs: FleetRecovery{GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum, Checksum: fleetRecoveryChecksum(data), Data: data}}})
	request.Selected = 1
	if err := store.Commit(t.Context(), generation, request.Inputs[0].Generation.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, append(opts, WithStateDirectory(request.Directory), WithGenerationPin(generation.Manifest.GenerationID), WithClientOptions(starmap.WithCatalogStore(store)))...)
	receipt, durable := r.PinAcceptance()
	if !durable || receipt != pin.Receipt {
		t.Fatal("original pin receipt changed")
	}
	if r.layers.publisherID != layers.publisherID || r.schedule.identity.Instance == layers.publisherID {
		t.Fatal("catalog lineage replaced native scheduler identity")
	}
	if r.permissions.required.Version != 0 || r.State().AuthorityHead != (catalogs.CatalogAuthorityHead{}) || r.config.origin != nil {
		t.Fatal("materialization fabricated serving authority")
	}
	if r.layers.manual == nil || !reflect.DeepEqual(r.layers.removals, layers.removals) {
		t.Fatal("historical private layers changed")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, append(opts, WithStateDirectory(request.Directory), WithClientOptions(starmap.WithCatalogStore(store)))...)
	if !restarted.Catalog().Removals().ContainsCanonical("author/current") {
		t.Fatal("unpin discarded original operator removal")
	}
	if restarted.layers.publisherID != layers.publisherID || restarted.State().PayloadChecksum != generation.Manifest.Payload.Checksum {
		t.Fatal("unpin changed reconstructed catalog lineage")
	}
}

func TestCatalogMaterializationInterruptedRemovalPreservesAbsence(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	prior := request.Inputs[0].Generation
	source := sourceLayer{Manifest: &prior.Manifest, Identity: "old-source", GenerationID: prior.Manifest.GenerationID, Checksum: prior.Manifest.Payload.Checksum, Payload: prior.Payload, PublishedAt: prior.Manifest.GeneratedAt}
	if err := store.saveSource(t.Context(), source); err != nil {
		t.Fatal(err)
	}
	ctx := cutMaterializationContext{Context: t.Context(), marker: filepath.Join(request.Directory, layerDirectoryName, materializationSelectionName)}
	if _, err := MaterializeCatalogRecovery(ctx, request, opts...); err == nil {
		t.Fatal("missing interruption")
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal(err)
	}
	restored, err := store.loadSource()
	if err != nil || restored != nil {
		t.Fatal("obsolete source became an empty record", err)
	}
	if _, err := os.Stat(filepath.Join(request.Directory, layerDirectoryName, sourceLayerFileName)); !os.IsNotExist(err) {
		t.Fatal("obsolete source file survived", err)
	}
	seed, err := os.ReadFile(filepath.Join(request.Directory, instanceSeedFileName))
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte(strings.Repeat("0", len(seed)))
	if err := os.WriteFile(filepath.Join(request.Directory, instanceSeedFileName), replacement, ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("completed retry accepted another replica seed")
	}
}

func TestCatalogMaterializationRetainsDistinctInputsForSameManifest(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	alternative := request.Inputs[0]
	record, err := readFleetRecovery(t.Context(), alternative.Recovery.Inputs.Data)
	if err != nil {
		t.Fatal(err)
	}
	record.PublisherID = "original-alternative-publisher"
	alternative.Recovery.Inputs.Data, err = encodeFleetRecoveryRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	alternative.Recovery.Inputs.Checksum = fleetRecoveryChecksum(alternative.Recovery.Inputs.Data)
	request.Inputs = append(request.Inputs, alternative)
	request.Selected = 1
	receipt, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SelectedInputsSHA256 != alternative.Recovery.Inputs.Checksum || receipt.CatalogPublisherID != record.PublisherID {
		t.Fatal("selection lost its exact input identity")
	}
	checksums, err := CatalogRecoveryChecksums(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity, alternative.Generation)
	if err != nil || len(checksums) != 2 {
		t.Fatal("alternate historical input capsule lost", checksums, err)
	}
}

func TestCatalogMaterializationRefusesUnknownFilesAfterInterruption(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	ctx := cutMaterializationContext{Context: t.Context(), marker: filepath.Join(request.Directory, layerDirectoryName, materializationSelectionName)}
	if _, err := MaterializeCatalogRecovery(ctx, request, opts...); err == nil {
		t.Fatal("missing interruption")
	}
	unknown := filepath.Join(request.Directory, layerDirectoryName, "operator-notes.txt")
	if err := os.WriteFile(unknown, []byte("operator work"), ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("unknown target content accepted")
	}
	data, err := os.ReadFile(unknown)
	if err != nil || string(data) != "operator work" {
		t.Fatal("unknown target content changed", err)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal("explicit owner cleanup did not permit recovery", err)
	}
}

func TestCatalogMaterializationExplicitRestoreRebindsOnlyNewOperation(t *testing.T) {
	request, opts, store := materializationFixture(t)
	first, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil {
		t.Fatal(err)
	}
	oldReceiptPath := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationReceiptName)
	originalReceipt, err := os.ReadFile(oldReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	archived := request.Directory + "-archived"
	if err := os.Rename(request.Directory, archived); err != nil {
		t.Fatal(err)
	}
	if _, err := privatefiles.NewDirectory(request.Directory); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(archived, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(archived, name)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(request.Directory, relative)
		if entry.IsDir() {
			_, err := privatefiles.NewDirectory(target)
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		parent, err := privatefiles.ExistingDirectory(filepath.Dir(target))
		if err != nil {
			return err
		}
		return parent.WriteFile(filepath.Base(target), data, ".fixture-")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory), WithClientOptions(starmap.WithCatalogStore(store)))...); err == nil {
		t.Fatal("copied native receipt admitted automatic startup")
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err == nil {
		t.Fatal("old operation accepted another native directory")
	}
	request.OperationID = "explicit-restored-target"
	second, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if second.DirectoryIdentity == first.DirectoryIdentity || second.SelectedManifestSHA256 != first.SelectedManifestSHA256 || second.SelectedInputsSHA256 != first.SelectedInputsSHA256 {
		t.Fatal("new operation lost target or selection binding")
	}
	retained, err := os.ReadFile(oldReceiptPath)
	if err != nil || !bytes.Equal(retained, originalReceipt) {
		t.Fatal("new recovery changed the original receipt", err)
	}
	r := openTestRuntime(t, append(opts, WithStateDirectory(request.Directory), WithClientOptions(starmap.WithCatalogStore(store)))...)
	if r.Status().RecoveryBaselineSHA256 != second.BaselineManifestSHA256 || r.permissions.required.Version != 0 || r.config.origin != nil {
		t.Fatal("explicit target rebind changed baseline or created authority")
	}
}

func TestCatalogMaterializationDoesNotStartConfiguredExternalRoles(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	source := newStubSource("retained-source")
	providers := &stubAcquirer{}
	leases := &stubLeaseStore{}
	metadataCalls, clockCalls := 0, 0
	metadata := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		metadataCalls++
		return nil, nil
	})
	opts = append(opts, WithSource(source), WithAcquisitionEnabled(true), WithAcquirer(providers), WithSourceAcquirer(metadata), WithLeaseStore(leases), WithClock(func() time.Time {
		clockCalls++
		return time.Now()
	}))
	config := defaults()
	if _, err := config.apply(opts...); err != nil {
		t.Fatal(err)
	}
	config.resolve()
	record, err := readFleetRecovery(t.Context(), request.Inputs[0].Recovery.Inputs.Data)
	if err != nil {
		t.Fatal(err)
	}
	layers, err := decodeFleetRecoveryRecord(t.Context(), record, layerSet{})
	if err != nil {
		t.Fatal(err)
	}
	layers.sourceConfiguration, err = describeSources(config)
	if err != nil {
		t.Fatal(err)
	}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Inputs[0].Recovery.Inputs.Data = data
	request.Inputs[0].Recovery.Inputs.Checksum = fleetRecoveryChecksum(data)
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal(err)
	}
	if source.readCount() != 0 || providers.callCount() != 0 || leases.acquireCount() != 0 || metadataCalls != 0 || clockCalls != 0 {
		t.Fatal("offline materialization started a configured external role")
	}
}

func TestCatalogMaterializationRefusesMissingActiveSelection(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	receipt, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(request.Directory, layerDirectoryName, materializationSelectionName)); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory))...); err == nil {
		t.Fatal("recovery baseline without active selection allowed startup")
	}
	retry, err := MaterializeCatalogRecovery(t.Context(), request, opts...)
	if err != nil || retry != receipt {
		t.Fatal("completed operation receipt was lost", err)
	}
	if _, err := os.Stat(filepath.Join(request.Directory, layerDirectoryName, materializationSelectionName)); !os.IsNotExist(err) {
		t.Fatal("completed retry reapplied its selection", err)
	}
}
