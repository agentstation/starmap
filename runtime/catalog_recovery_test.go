package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRuntimeRetainsGenerationInputsBeforePublication(t *testing.T) {
	directory := privateRuntimeDirectory(t)
	store := &recoveryCheckingStore{Store: storage.NewMemory(), directory: directory, t: t}
	r := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithCatalogNetworkMode("offline"), WithAcquisitionEnabled(false), WithRetentionEnabled(false), WithClientOptions(starmap.WithCatalogStore(store)))
	if r.State().GenerationID == "" {
		t.Fatal("missing generation")
	}
}

type recoveryCheckingStore struct {
	storage.Store
	directory string
	t         *testing.T
}

func (s *recoveryCheckingStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	names, err := filepath.Glob(filepath.Join(s.directory, "catalog-runtime", "generation-inputs", "*.json.gz"))
	if err != nil || len(names) == 0 {
		s.t.Fatalf("catalog commit has no prior generation-bound input record: %v", err)
	}
	return s.Store.Commit(ctx, generation, expected)
}

func TestCatalogRecoveryKeepsAcceptedAndCandidateInputs(t *testing.T) {
	store := storage.NewMemory()
	r := smallCatalogRecoveryRuntime(t, store)
	var opts []Option
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	before, err := CatalogRecoveryChecksums(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, accepted)
	if err != nil || len(before) != 1 {
		t.Fatalf("accepted inputs: %v %v", before, err)
	}
	layers := r.layers
	observations, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "later", accepted.Manifest.GeneratedAt.Add(time.Minute), false)})
	if err != nil {
		t.Fatal(err)
	}
	layers.manual = &manualBatch{observations: observations}
	state, err := layers.build(t.Context(), layers.embedded)
	if err != nil {
		t.Fatal(err)
	}
	update, err := starmap.NewCandidate(state.Catalog, layers.buildEvidence, starmap.WithCandidateGenerationID(state.GenerationID))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := r.client.PrepareGeneration(t.Context(), update)
	if err != nil {
		t.Fatal(err)
	}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(t.Context(), localRecoveryContextKey{}, localRecoveryTestAttempt(t, r.store, data))
	if _, err := r.client.Activate(ctx, prepared); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	after, err := CatalogRecoveryChecksums(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, candidate)
	if err != nil || len(after) != 1 {
		t.Fatalf("candidate inputs: %v %v", after, err)
	}
	baselines, err := filepath.Glob(filepath.Join(r.config.stateDirectory, layerDirectoryName, generationBaselinesDirectory, "*.json.gz"))
	if err != nil || len(baselines) != 1 {
		t.Fatalf("two generations must share one immutable baseline: %v %v", baselines, err)
	}
	inputs, err := filepath.Glob(filepath.Join(r.config.stateDirectory, layerDirectoryName, generationInputsDirectory, "*.json.gz"))
	if err != nil || len(inputs) != 2 {
		t.Fatalf("two generations must retain separate inputs: %v %v", inputs, err)
	}
	for _, input := range inputs {
		data, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decompressFleetRecovery(t.Context(), data, maxCatalogRecoveryBytes)
		if err != nil || bytes.Contains(decoded, []byte(`"baseline":`)) {
			t.Fatalf("generation descriptor duplicates its baseline: %v", err)
		}
	}
	if accepted.Manifest.GenerationID == candidate.Manifest.GenerationID || before[0] == after[0] {
		t.Fatal("distinct generations lost input identity")
	}
	for _, item := range []struct {
		generation catalogs.Generation
		checksum   string
	}{{accepted, before[0]}, {candidate, after[0]}} {
		record, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, item.generation, item.checksum)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateCatalogReplay(t.Context(), item.generation, record, opts...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, accepted, after[0]); err == nil {
		t.Fatal("candidate inputs replaced accepted history")
	}
}

func TestCatalogRecoveryDoesNotInferDirectMutationHistory(t *testing.T) {
	store := storage.NewMemory()
	r := smallCatalogRecoveryRuntime(t, store)
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.GenerationID += "-direct"
	if _, err := r.Client().Activate(t.Context(), generation); err != nil {
		t.Fatal(err)
	}
	checksums, err := CatalogRecoveryChecksums(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation)
	if err != nil || len(checksums) != 0 {
		t.Fatalf("direct publication acquired inferred input evidence: %v %v", checksums, err)
	}
}

func TestCatalogRecoveryExactRetryAndRefusal(t *testing.T) {
	store := storage.NewMemory()
	r := smallCatalogRecoveryRuntime(t, store)
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checksums, err := CatalogRecoveryChecksums(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation)
	if err != nil || len(checksums) != 1 {
		t.Fatal(err)
	}
	record, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation, checksums[0])
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(t.Context(), localRecoveryContextKey{}, localRecoveryTestAttempt(t, r.store, record.Inputs.Data))
	if err := retainLocalGeneration(ctx, generation); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCatalogRecovery(nil, r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation, checksums[0]); err == nil {
		t.Fatal("nil context accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := retainLocalGeneration(cancelled, generation); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
	changed := generation.Copy()
	changed.Manifest.GenerationID += "-foreign"
	if err := ValidateCatalogReplay(t.Context(), changed, record); err == nil {
		t.Fatal("foreign generation accepted")
	}
	record.Inputs.Data = append([]byte(nil), record.Inputs.Data...)
	record.Inputs.Data[0] ^= 1
	if err := ValidateCatalogReplay(t.Context(), generation, record); err == nil {
		t.Fatal("corrupt inputs accepted")
	}
}

func TestCatalogRecoveryCollectsOnlyUncommittedOrphans(t *testing.T) {
	store := storage.NewMemory()
	r := smallCatalogRecoveryRuntime(t, store)
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checksums, err := CatalogRecoveryChecksums(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation)
	if err != nil || len(checksums) != 1 {
		t.Fatal(err)
	}
	record, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation, checksums[0])
	if err != nil {
		t.Fatal(err)
	}
	orphan := generation.Copy()
	orphan.Manifest.GenerationID += "-uncommitted"
	ctx := context.WithValue(t.Context(), localRecoveryContextKey{}, localRecoveryTestAttempt(t, r.store, record.Inputs.Data))
	if err := retainLocalGeneration(ctx, orphan); err != nil {
		t.Fatal(err)
	}
	r.publicationMu.Lock()
	r.providerRetentionMu.Lock()
	_, _, removed, err := r.collectCatalogRecovery(t.Context(), storage.DefaultRetentionScanEntries, storage.DefaultRetentionInputMaxBytes)
	r.providerRetentionMu.Unlock()
	r.publicationMu.Unlock()
	if err != nil || removed != 1 {
		t.Fatalf("orphan collection: removed=%d err=%v", removed, err)
	}
	if _, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation, checksums[0]); err != nil {
		t.Fatal("collection removed retained generation inputs", err)
	}
	if _, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, orphan, checksums[0]); !os.IsNotExist(err) {
		t.Fatal("uncommitted capsule survived collection", err)
	}
}

func TestCatalogRecoveryInspectionRejectsChangedImmutableRecord(t *testing.T) {
	store := storage.NewMemory()
	r := smallCatalogRecoveryRuntime(t, store)
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	checksums, err := CatalogRecoveryChecksums(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation)
	if err != nil || len(checksums) != 1 {
		t.Fatal(err)
	}
	manifest, err := catalogManifestChecksum(generation)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.config.stateDirectory, layerDirectoryName, generationInputsDirectory, manifest+"-"+checksums[0]+".json.gz")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decompressFleetRecovery(t.Context(), original, maxCatalogRecoveryBytes)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(decoded, []byte(`"manifest_checksum":`), []byte(`"unknown":true,"manifest_checksum":`), 1)
	changed, err = compressFleetRecovery(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedDirectory(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity); err == nil {
		t.Fatal("inspection accepted unknown recovery schema")
	}
	if err := os.WriteFile(path, original, ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCatalogRecovery(t.Context(), r.config.stateDirectory, r.config.directoryOwner, r.config.schedulerIdentity, generation, checksums[0]); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRecoveryFleetCapturePreservesOriginalSelection(t *testing.T) {
	snapshot, _ := fleetValidationFixture(t)
	recovery, err := CaptureFleetCatalogRecovery(t.Context(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCatalogReplay(t.Context(), snapshot.Publication.Generation, recovery); err != nil {
		t.Fatal(err)
	}
	recovery.Inputs.Data[0] ^= 1
	if err := ValidateFleetRecovery(t.Context(), snapshot); err != nil {
		t.Fatal("capture changed original fleet inputs", err)
	}
	snapshot.Head.Revision++
	if _, err := CaptureFleetCatalogRecovery(t.Context(), snapshot); err == nil {
		t.Fatal("captured inconsistent fleet selection")
	}
}

// smallCatalogRecoveryRuntime uses native private files and real catalog commits.
// Only the embedded test baseline is smaller than the product catalog.
func smallCatalogRecoveryRuntime(t *testing.T, store storage.Store) *Runtime {
	t.Helper()
	snapshot, layers := fleetValidationFixture(t)
	config := defaults()
	config.resolve()
	config.stateDirectory = privateRuntimeDirectory(t)
	if err := bindDirectoryOwner(t.Context(), config.stateDirectory, config.directoryOwner, config.schedulerIdentity); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareInstanceSeed(t.Context(), config.stateDirectory); err != nil {
		t.Fatal(err)
	}
	files, err := newLayerStore(config.stateDirectory)
	if err != nil {
		t.Fatal(err)
	}
	client, err := starmap.New(starmap.WithCatalogStore(store), starmap.WithGenerationRetainer(retainLocalGeneration))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(t.Context(), localRecoveryContextKey{}, localRecoveryTestAttempt(t, files, snapshot.Publication.Recovery.Data))
	if _, err := client.Activate(ctx, snapshot.Publication.Generation); err != nil {
		t.Fatal(err)
	}
	return &Runtime{client: client, store: files, config: *config, layers: layers, ctx: t.Context()}
}

func localRecoveryTestAttempt(t *testing.T, store *layerStore, data []byte) *localRecoveryAttempt {
	t.Helper()
	record, err := readFleetRecovery(t.Context(), data)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := encodeCatalogRecoveryBaseline(record.Baseline)
	if err != nil {
		t.Fatal(err)
	}
	record.Baseline = catalogs.Generation{}
	return &localRecoveryAttempt{store: store, inputs: record, baseline: baseline}
}

func TestCatalogRecoveryCodecPreservesPublicationPolicyDuration(t *testing.T) {
	_, layers := fleetValidationFixture(t)
	upstream := sourcePublicationFixture(t)
	generation := upstream.Generation
	layers.source = &sourceLayer{Identity: "upstream", GenerationID: generation.Manifest.GenerationID,
		Checksum: generation.Manifest.Payload.Checksum, Payload: generation.Payload, Manifest: &generation.Manifest,
		PublishedAt: generation.Manifest.GeneratedAt, Publication: upstream.Publication}
	encoded, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := readFleetRecovery(t.Context(), encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Source == nil || !reflect.DeepEqual(decoded.Source.Publication, upstream.Publication) {
		t.Fatal("fleet recovery changed the upstream receipt or its nanosecond policy")
	}
}
