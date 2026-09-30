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
	"github.com/agentstation/starmap/pkg/sources"
)

func retentionFixture(t *testing.T, count int) (CatalogRetentionRequest, []Option) {
	t.Helper()
	original, opts, _ := materializationFixture(t)
	request := CatalogRetentionRequest{Directory: original.Directory, Owner: original.Owner, SchedulerIdentity: original.SchedulerIdentity, OperationID: "retain-1", TransferID: "transfer-test"}
	for i := range count {
		input := original.Inputs[0]
		input.Generation = input.Generation.Copy()
		input.Generation.Manifest.GenerationID += strings.Repeat("x", i+1)
		input.Recovery.ManifestChecksum, _ = catalogManifestChecksum(input.Generation)
		input.Recovery.Inputs.GenerationID = input.Generation.Manifest.GenerationID
		request.Manifest = append(request.Manifest, CatalogRetentionEntry{ManifestSHA256: input.Recovery.ManifestChecksum, InputsSHA256: input.Recovery.Inputs.Checksum, SourceOrigin: CatalogRetentionNoDescriptor})
		request.Inputs = append(request.Inputs, CatalogRetentionInput{CatalogMaterializationInput: input})
	}
	return request, opts
}
func retentionBatchReference(t *testing.T, receipt CatalogRetentionReceipt) CatalogRetentionBatch {
	t.Helper()
	raw, err := json.Marshal(receipt, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	return CatalogRetentionBatch{OperationID: receipt.OperationID, ReceiptSHA256: fleetRecoveryChecksum(raw)}
}
func retentionSelection(request CatalogRetentionRequest, batches []CatalogRetentionBatch) CatalogRetainedMaterializationRequest {
	return CatalogRetainedMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: "select-test", TransferID: request.TransferID, Batches: batches, Selected: 0}
}
func TestCatalogRetentionKeepsHistoricSettingsWithoutSelection(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	input := &request.Inputs[0]
	record, err := readFleetRecovery(t.Context(), input.Recovery.Inputs.Data)
	if err != nil {
		t.Fatal(err)
	}
	record.Compatibility = strings.Repeat("c", 64)
	input.Recovery.Inputs.Data, err = encodeFleetRecoveryRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	input.Recovery.Inputs.Checksum = fleetRecoveryChecksum(input.Recovery.Inputs.Data)
	request.Manifest[0].InputsSHA256 = input.Recovery.Inputs.Checksum
	if err := ValidateCatalogReplay(t.Context(), input.Generation, input.Recovery, opts...); err == nil {
		t.Fatal("fixture did not require historical settings")
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	before, err := captureMaterializationFiles(t.Context(), store.directory)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Validation != "structural" {
		t.Fatal("historical retention claimed semantic replay")
	}
	after, err := captureMaterializationFiles(t.Context(), store.directory)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("retention changed active inputs", err)
	}
	directory, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := readRetainedCatalogInput(t.Context(), directory, receipt.Records[0])
	if err != nil || !reflect.DeepEqual(retained, input.CatalogMaterializationInput) {
		t.Fatal("original recovery bytes changed", err)
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)})
	if _, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...); err == nil {
		t.Fatal("incompatible historical input was selected")
	}
	if _, err := os.Stat(filepath.Join(request.Directory, layerDirectoryName, recoveryBaselineName)); !os.IsNotExist(err) {
		t.Fatal("failed selection switched baseline", err)
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory))...)
	if err != nil {
		t.Fatal("completed structural retention blocked ordinary startup", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestCatalogRetentionBatchesComplete96EntryInventory(t *testing.T) {
	request, opts := retentionFixture(t, 96)
	inputs := request.Inputs
	var batches []CatalogRetentionBatch
	for start := 0; start < len(inputs); start += 12 {
		batch := request
		batch.OperationID += "-" + strings.Repeat("x", start+1)
		batch.BatchStart = start
		batch.Inputs = inputs[start : start+12]
		receipt, err := RetainCatalogRecovery(t.Context(), batch)
		if err != nil {
			t.Fatal(err)
		}
		batches = append(batches, retentionBatchReference(t, receipt))
	}
	selected := retentionSelection(request, batches)
	selected.Selected = 95
	receipt, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SelectedManifestSHA256 != request.Manifest[95].ManifestSHA256 {
		t.Fatal("wrong final selected generation")
	}
	retry, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil || receipt != retry {
		t.Fatal("exact selected retry changed receipt", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*CatalogRetainedMaterializationRequest)
	}{
		{"gap", func(r *CatalogRetainedMaterializationRequest) { r.Batches = r.Batches[1:] }},
		{"overlap", func(r *CatalogRetainedMaterializationRequest) { r.Batches = append(r.Batches, r.Batches[0]) }},
		{"reordered", func(r *CatalogRetainedMaterializationRequest) {
			r.Batches[0], r.Batches[1] = r.Batches[1], r.Batches[0]
		}},
		{"settings", func(r *CatalogRetainedMaterializationRequest) { r.TransferID += "-changed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := selected
			changed.Batches = append([]CatalogRetentionBatch(nil), selected.Batches...)
			test.mutate(&changed)
			changed.OperationID += "-new"
			if _, err := MaterializeRetainedCatalogRecovery(t.Context(), changed, opts...); err == nil {
				t.Fatal("invalid batch coverage accepted")
			}
		})
	}
}
func TestCatalogRetentionCompletedRetryDoesNotRecreateRecords(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)})
	completion, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil {
		t.Fatal(err)
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("{\"version\":1,\"phase\":\"idle\"}\n")
	if err := store.directory.PublishFileContext(t.Context(), inputPublicationName, marker, ".layer-"); err != nil {
		t.Fatal(err)
	}
	root, err := directory.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Remove(receipt.Records[0].RecordSHA256 + ".json.gz"); err != nil {
		t.Fatal(err)
	}
	if err := root.Remove("manifest.json"); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	retried, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil || !reflect.DeepEqual(receipt, retried) {
		t.Fatal("completed retention retry", err)
	}
	materialized, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil || materialized != completion {
		t.Fatal("completed selected retry", err)
	}
	actual, err := store.directory.ReadFile(inputPublicationName, maxLayerBytes)
	if err != nil || !bytes.Equal(actual, marker) {
		t.Fatal("completed retry replaced later state", err)
	}
	if _, err := directory.ReadFile("manifest.json", maxLayerBytes); !os.IsNotExist(err) {
		t.Fatal("retry recreated missing manifest", err)
	}
	selected.OperationID += "-different"
	if _, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...); err == nil {
		t.Fatal("new selection ignored missing original evidence")
	}
	if _, err := MaterializeRetainedCatalogRecovery(t.Context(), retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)}), WithCatalogSource("github")); err == nil {
		t.Fatal("changed target settings reused old completion")
	}
}
func TestCatalogRetentionInterruptionRefusesChangedInputsAndCurrentState(t *testing.T) {
	for _, point := range []string{"plan", "envelope", "complete"} {
		t.Run(point, func(t *testing.T) {
			request, opts := retentionFixture(t, 1)
			marker := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationPlanName)
			if point == "envelope" {
				data, err := encodeRetainedCatalogInput(request.Manifest[0], request.Inputs[0])
				if err != nil {
					t.Fatal(err)
				}
				marker = filepath.Join(request.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(request.TransferID)), fleetRecoveryChecksum(data)+".json.gz")
			}
			if point == "complete" {
				marker = filepath.Join(filepath.Dir(marker), materializationReceiptName)
			}
			ctx := cutMaterializationContext{Context: t.Context(), marker: marker}
			if _, err := RetainCatalogRecovery(ctx, request); err == nil {
				t.Fatal("interruption did not stop retention")
			}
			if point != "complete" {
				if r, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory))...); err == nil {
					_ = r.Close()
					t.Fatal("pending retention allowed runtime startup")
				}
			}
			changed := request
			changed.TransferID += "-changed"
			if _, err := RetainCatalogRecovery(t.Context(), changed); err == nil {
				t.Fatal("changed operation inputs accepted")
			}
			if _, err := RetainCatalogRecovery(t.Context(), request); err != nil {
				t.Fatal("exact resume failed", err)
			}
		})
	}
	request, _ := retentionFixture(t, 1)
	marker := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationPlanName)
	if _, err := RetainCatalogRecovery(cutMaterializationContext{Context: t.Context(), marker: marker}, request); err == nil {
		t.Fatal("interruption fixture failed")
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.directory.PublishFileContext(t.Context(), inputPublicationName, []byte("{}"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	if _, err := RetainCatalogRecovery(t.Context(), request); err == nil {
		t.Fatal("changed current state accepted")
	}
}
func TestCatalogRetentionPreservesExactLocalDescriptor(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	materialize := CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: "original-local", Inputs: []CatalogMaterializationInput{request.Inputs[0].CatalogMaterializationInput}}
	if _, err := MaterializeCatalogRecovery(t.Context(), materialize, opts...); err != nil {
		t.Fatal(err)
	}
	checksums, err := CatalogRecoveryChecksums(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity, request.Inputs[0].Generation)
	if err != nil || len(checksums) != 1 {
		t.Fatal(err)
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := store.directory.ExistingChild(generationInputsDirectory)
	if err != nil {
		t.Fatal(err)
	}
	original, err := directory.ReadFile(request.Manifest[0].ManifestSHA256+"-"+checksums[0]+".json.gz", MaxFleetRecoveryBytes)
	if err != nil {
		t.Fatal(err)
	}
	request.Manifest[0].SourceOrigin = CatalogRetentionLocalDescriptor
	request.Manifest[0].SourceDescriptorSHA256 = checksums[0]
	request.Inputs[0].SourceDescriptor = original
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := retained.ReadFile(receipt.Records[0].RecordSHA256+".json.gz", maxRetainedCatalogRecordBytes)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decompressRecoveryRecord(t.Context(), raw, maxRetainedCatalogRecordBytes, maxRetainedCatalogRecordBytes)
	if err != nil {
		t.Fatal(err)
	}
	var envelope retainedCatalogEnvelope
	if err := json.Unmarshal(decoded, &envelope); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, envelope.Input.SourceDescriptor) || !bytes.Equal(request.Inputs[0].Recovery.Inputs.Data, envelope.Input.Recovery.Inputs.Data) {
		t.Fatal("retention rewrote original bytes")
	}
	request.OperationID += "-bad"
	request.Inputs[0].SourceDescriptor = []byte("changed")
	if _, err := RetainCatalogRecovery(t.Context(), request); err == nil {
		t.Fatal("changed descriptor accepted")
	}
}
func TestCatalogRetentionRefusesManifestChangesAndUnknownFiles(t *testing.T) {
	request, _ := retentionFixture(t, 2)
	all := request.Inputs
	request.Inputs = all[:1]
	if _, err := RetainCatalogRecovery(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.OperationID += "-2"
	changed.Manifest = append([]CatalogRetentionEntry(nil), request.Manifest...)
	changed.Manifest[1].SourceOrigin = CatalogRetentionFleetDescriptor
	changed.Manifest[1].SourceDescriptorSHA256 = strings.Repeat("a", 64)
	if _, err := RetainCatalogRecovery(t.Context(), changed); err == nil {
		t.Fatal("full manifest changed between batches")
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.PublishFileContext(t.Context(), "unknown.json", []byte("{}"), ".retained-input-"); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err == nil {
		t.Fatal("unknown retention artifact accepted")
	}
}
func TestCatalogRetentionPrivateRecordsAndSourceOriginBounds(t *testing.T) {
	request, _ := retentionFixture(t, 1)
	for _, change := range []func(*CatalogRetentionRequest){
		func(r *CatalogRetentionRequest) { r.Manifest[0].SourceOrigin = CatalogRetentionLocalDescriptor },
		func(r *CatalogRetentionRequest) { r.Inputs[0].SourceDescriptor = []byte("unexpected") },
		func(r *CatalogRetentionRequest) { r.BatchStart = 1 },
		func(r *CatalogRetentionRequest) { r.BatchStart = int(^uint(0) >> 1) },
		func(r *CatalogRetentionRequest) { r.OperationID = "\x00" },
	} {
		changed := request
		changed.Manifest = append([]CatalogRetentionEntry(nil), request.Manifest...)
		changed.Inputs = append([]CatalogRetentionInput(nil), request.Inputs...)
		change(&changed)
		if _, err := RetainCatalogRecovery(t.Context(), changed); err == nil {
			t.Fatal("invalid source or range accepted")
		}
	}
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := privatefiles.ExistingDirectory(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectRuntimeInventory(t.Context(), directory); err != nil {
		t.Fatal("private retention inspection failed", err)
	}
	if receipt.DirectoryIdentity.Access == "" {
		t.Fatal("retention has no native access evidence")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := RetainCatalogRecovery(canceled, request); err == nil {
		t.Fatal("canceled request succeeded")
	}
	if _, err := RetainCatalogRecovery(nil, request); err == nil {
		t.Fatal("nil context succeeded")
	}
}

func TestCatalogRetentionReaderRefusesChangedScopeAndReturnsPrivateCopies(t *testing.T) {
	request, _ := retentionFixture(t, 1)
	descriptor := []byte("{\"original_publication\":\"host-verified\"}")
	request.Manifest[0].SourceOrigin = CatalogRetentionFleetDescriptor
	request.Manifest[0].SourceDescriptorSHA256 = fleetRecoveryChecksum(descriptor)
	request.Inputs[0].SourceDescriptor = descriptor
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	read := CatalogRetainedReadRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, TransferID: request.TransferID, Batch: retentionBatchReference(t, receipt), Index: 0}
	first, err := ReadRetainedCatalogRecovery(t.Context(), read)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, request.Inputs[0]) {
		t.Fatal("read changed original descriptor or exported inputs")
	}
	first.SourceDescriptor[0] = 'x'
	first.Recovery.Inputs.Data[0] = 'x'
	first.Generation.Payload[0] = 'x'
	second, err := ReadRetainedCatalogRecovery(t.Context(), read)
	if err != nil || !reflect.DeepEqual(second, request.Inputs[0]) {
		t.Fatal("caller mutation changed retained evidence", err)
	}
	for _, change := range []func(*CatalogRetainedReadRequest){
		func(r *CatalogRetainedReadRequest) { r.Index = 1 },
		func(r *CatalogRetainedReadRequest) { r.Index = int(^uint(0) >> 1) },
		func(r *CatalogRetainedReadRequest) { r.Owner.Deployment += "-changed" },
		func(r *CatalogRetainedReadRequest) { r.TransferID += "-changed" },
		func(r *CatalogRetainedReadRequest) { r.Batch.ReceiptSHA256 = strings.Repeat("a", 64) },
	} {
		changed := read
		change(&changed)
		if _, err := ReadRetainedCatalogRecovery(t.Context(), changed); err == nil {
			t.Fatal("changed native owner, receipt, or scope accepted")
		}
	}
	path := filepath.Join(request.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(request.TransferID)), receipt.Records[0].RecordSHA256+".json.gz")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRetainedCatalogRecovery(t.Context(), read); err == nil {
		t.Fatal("passive read repaired missing envelope")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("passive read recreated envelope", err)
	}
}

func TestCatalogRetentionPreservesPermissionAndIdentityContinuity(t *testing.T) {
	request, _ := retentionFixture(t, 1)
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	permission := authorityPermissionFixture()
	state, err := (authorityPermissions{authorityID: permission.Head.AuthorityID, policyID: permission.Head.PolicyID}).observe(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.savePermission(t.Context(), state, true); err != nil {
		t.Fatal(err)
	}
	before, err := store.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.directory.ReadFile(permissionCheckpointFile, maxPermissionCheckpointBytes)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("retention changed permission evidence", err)
	}
	request.OperationID += "-pending"
	marker := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationPlanName)
	if _, err := RetainCatalogRecovery(cutMaterializationContext{Context: t.Context(), marker: marker}, request); err == nil {
		t.Fatal("interruption fixture failed")
	}
	if err := store.savePermission(t.Context(), state, false); err != nil {
		t.Fatal(err)
	}
	if _, err := RetainCatalogRecovery(t.Context(), request); err == nil {
		t.Fatal("pending retention accepted changed permission continuity")
	}
	request.OperationID = receipt.OperationID
	retry, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil || !reflect.DeepEqual(receipt, retry) {
		t.Fatal("historical retry required old permission or changed selection", err)
	}
}

func TestCatalogRetentionRecoversProcessExit(t *testing.T) {
	if source := os.Getenv("STARMAP_TEST_RETENTION_REQUEST"); source != "" {
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		var request CatalogRetentionRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Fatal(err)
		}
		_, err = RetainCatalogRecovery(exitMaterializationContext{Context: t.Context(), marker: os.Getenv("STARMAP_TEST_RETENTION_MARKER")}, request)
		t.Fatalf("process did not exit: %v", err)
	}
	for _, point := range []string{"plan", "envelope"} {
		t.Run(point, func(t *testing.T) {
			request, opts := retentionFixture(t, 1)
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "request.json")
			if err := os.WriteFile(source, raw, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationPlanName)
			if point == "envelope" {
				data, err := encodeRetainedCatalogInput(request.Manifest[0], request.Inputs[0])
				if err != nil {
					t.Fatal(err)
				}
				marker = filepath.Join(request.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(request.TransferID)), fleetRecoveryChecksum(data)+".json.gz")
			}
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCatalogRetentionRecoversProcessExit$")
			command.Env = append(os.Environ(), "STARMAP_TEST_RETENTION_REQUEST="+source, "STARMAP_TEST_RETENTION_MARKER="+marker)
			output, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v %s", err, output)
			}
			if r, err := Open(t.Context(), append(opts, WithStateDirectory(request.Directory))...); err == nil {
				_ = r.Close()
				t.Fatal("abandoned retention allowed startup")
			}
			if _, err := RetainCatalogRecovery(t.Context(), request); err != nil {
				t.Fatal("process interruption recovery", err)
			}
		})
	}
}

func TestCatalogRetentionFullEmbeddedRoundtrip(t *testing.T) {
	request, opts := retentionFixture(t, 1)
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
	request.Inputs = []CatalogRetentionInput{{CatalogMaterializationInput: CatalogMaterializationInput{Generation: baseline, Recovery: CatalogRecovery{ManifestChecksum: manifest, Inputs: FleetRecovery{GenerationID: baseline.Manifest.GenerationID, PayloadChecksum: baseline.Manifest.Payload.Checksum, Checksum: fleetRecoveryChecksum(data), Data: data}}}}}
	request.Manifest = []CatalogRetentionEntry{{ManifestSHA256: manifest, InputsSHA256: fleetRecoveryChecksum(data), SourceOrigin: CatalogRetentionNoDescriptor}}
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeRetainedCatalogRecovery(t.Context(), retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)}), opts...); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRetentionInspectionRequiresCurrentCompleteSelection(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)})
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, CatalogMaterializationReceipt{}, opts...); err == nil {
		t.Fatal("inspection continued an absent materialization")
	}
	materialized, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, materialized, opts...); err != nil {
		t.Fatal("current complete selection inspection", err)
	}
	changed := materialized
	changed.PlanSHA256 = strings.Repeat("a", 64)
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, changed, opts...); err == nil {
		t.Fatal("changed expected completion accepted")
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, materialized, append(opts, WithAcquisitionSources())...); err == nil {
		t.Fatal("changed acquisition configuration reused completion")
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	later := []byte("{\"version\":1,\"phase\":\"idle\"}\n")
	if err := store.directory.PublishFileContext(t.Context(), inputPublicationName, later, ".layer-"); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, materialized, opts...); err == nil {
		t.Fatal("inspection accepted later changed active inputs")
	}
	retry, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil || retry != materialized {
		t.Fatal("historical retry stopped being passive", err)
	}
	actual, err := store.directory.ReadFile(inputPublicationName, maxLayerBytes)
	if err != nil || !bytes.Equal(actual, later) {
		t.Fatal("inspection or retry replaced later state", err)
	}
}

func TestCatalogRetentionNativeRestoreRequiresNewBatchOperation(t *testing.T) {
	request, _ := retentionFixture(t, 1)
	first, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)), materializationReceiptName)
	original, err := os.ReadFile(receiptPath)
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
	if _, err := RetainCatalogRecovery(t.Context(), request); err == nil {
		t.Fatal("old retention operation adopted changed native directory")
	}
	read := CatalogRetainedReadRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, TransferID: request.TransferID, Batch: retentionBatchReference(t, first)}
	if _, err := ReadRetainedCatalogRecovery(t.Context(), read); err == nil {
		t.Fatal("old batch read adopted changed native directory")
	}
	request.OperationID += "-restored"
	second, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal("new explicit restored batch", err)
	}
	if first.DirectoryIdentity == second.DirectoryIdentity {
		t.Fatal("new operation lost native target binding")
	}
	retained, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(original, retained) {
		t.Fatal("new operation replaced historical receipt", err)
	}
}

func TestCatalogRetentionEncodedBoundsAndMissingHistoricalEvidence(t *testing.T) {
	if _, err := marshalCatalogRetention(strings.Repeat("x", 64), 8); err == nil {
		t.Fatal("encoded metadata exceeded its fixed bound")
	}
	request, opts := retentionFixture(t, 2)
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(request.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(request.TransferID)), receipt.Records[1].RecordSHA256+".json.gz")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)})
	if _, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...); err == nil {
		t.Fatal("complete coverage ignored missing unselected historical envelope")
	}
	pending, _ := retentionFixture(t, 1)
	marker := filepath.Join(pending.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(pending.OperationID)), materializationPlanName)
	if _, err := RetainCatalogRecovery(cutMaterializationContext{Context: t.Context(), marker: marker}, pending); err == nil {
		t.Fatal("pending fixture failed")
	}
	manifestPath := filepath.Join(pending.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(pending.TransferID)), "manifest.json")
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if _, err := RetainCatalogRecovery(t.Context(), pending); err == nil {
		t.Fatal("pending retention repaired a removed manifest")
	}
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatal("pending retention recreated changed evidence", err)
	}
}

func TestCatalogRetentionSelectionAndInspectionKeepExternalRolesPassive(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	source := newStubSource("retained-source")
	providers := &stubAcquirer{}
	leases := &stubLeaseStore{}
	metadataCalls, clockCalls := 0, 0
	metadata := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		metadataCalls++
		return nil, nil
	})
	opts = append(opts, WithSource(source), WithAcquisitionEnabled(true), WithAcquirer(providers), WithSourceAcquirer(metadata), WithLeaseStore(leases), WithClock(func() time.Time { clockCalls++; return time.Now() }))
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
	request.Manifest[0].InputsSHA256 = fleetRecoveryChecksum(data)
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, receipt)})
	completion, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, completion, opts...); err != nil {
		t.Fatal(err)
	}
	if source.readCount() != 0 || providers.callCount() != 0 || leases.acquireCount() != 0 || metadataCalls != 0 || clockCalls != 0 {
		t.Fatal("offline retention or selection started an external role")
	}
}

func TestCatalogRetentionInspectionRefusesPendingOrLaterSelection(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	retained, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{retentionBatchReference(t, retained)})
	first, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil {
		t.Fatal(err)
	}
	pending := request
	pending.OperationID += "-pending"
	marker := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(pending.OperationID)), materializationPlanName)
	if _, err := RetainCatalogRecovery(cutMaterializationContext{Context: t.Context(), marker: marker}, pending); err == nil {
		t.Fatal("pending retention fixture did not interrupt")
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, first, opts...); err == nil {
		t.Fatal("inspection accepted another pending recovery operation")
	}
	if _, err := RetainCatalogRecovery(t.Context(), pending); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, first, opts...); err != nil {
		t.Fatal("completed structural retention changed selection", err)
	}
	later := selected
	later.OperationID += "-later"
	second, err := MaterializeRetainedCatalogRecovery(t.Context(), later, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), selected, first, opts...); err == nil {
		t.Fatal("old operation claimed a later identical input selection")
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), later, second, opts...); err != nil {
		t.Fatal("current later selection refused", err)
	}
	retry, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...)
	if err != nil || retry != first {
		t.Fatal("old completed retry changed after later selection", err)
	}
	if err := InspectRetainedCatalogMaterialization(t.Context(), later, second, opts...); err != nil {
		t.Fatal("old completed retry replaced the later selection", err)
	}
}

func TestCatalogRetentionCompletedBatchRefusesReplacedValidSeed(t *testing.T) {
	request, opts := retentionFixture(t, 1)
	retained, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := privatefiles.ExistingDirectory(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := owner.ReadFile(instanceSeedFileName, instanceSeedBytes*2)
	if err != nil {
		t.Fatal(err)
	}
	replacement := bytes.Repeat([]byte("a"), len(seed))
	if bytes.Equal(seed, replacement) {
		replacement = bytes.Repeat([]byte("b"), len(seed))
	}
	if err := owner.WriteFile(instanceSeedFileName, replacement, ".fixture-"); err != nil {
		t.Fatal(err)
	}
	if err := inspectMaterializationOwner(t.Context(), owner, materializationOwnerRequest(request)); err != nil {
		t.Fatal("fixture did not retain a valid current owner and seed", err)
	}
	batch := retentionBatchReference(t, retained)
	read := CatalogRetainedReadRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, TransferID: request.TransferID, Batch: batch, Index: 0}
	if _, err := ReadRetainedCatalogRecovery(t.Context(), read); err == nil {
		t.Fatal("completed batch read accepted a replaced valid seed")
	}
	selected := retentionSelection(request, []CatalogRetentionBatch{batch})
	if _, err := MaterializeRetainedCatalogRecovery(t.Context(), selected, opts...); err == nil {
		t.Fatal("new selection accepted a batch from a replaced valid seed")
	}
	if _, err := os.Stat(filepath.Join(request.Directory, layerDirectoryName, recoveryBaselineName)); !os.IsNotExist(err) {
		t.Fatal("refused seed continuity changed active baseline", err)
	}
}
