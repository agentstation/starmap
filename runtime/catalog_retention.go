package runtime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	stderrors "errors"
	"github.com/gofrs/flock"
	"path/filepath"
	"reflect"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// CatalogRetentionOrigin identifies the supplied historical descriptor format.
// Descriptors remain evidence and confer no current permission.
type CatalogRetentionOrigin string

// Descriptor origin values distinguish original bytes from explicitly absent evidence.
const (
	CatalogRetentionLocalDescriptor CatalogRetentionOrigin = "local-descriptor"
	CatalogRetentionFleetDescriptor CatalogRetentionOrigin = "fleet-publication"
	CatalogRetentionNoDescriptor    CatalogRetentionOrigin = "no-descriptor"
)

// CatalogRetentionEntry binds original descriptor bytes and exported reconstruction inputs.
// InputsSHA256 is CatalogRecovery.Inputs.Checksum, not the local descriptor checksum.
type CatalogRetentionEntry struct {
	ManifestSHA256         string                 `json:"manifest_sha256"`
	InputsSHA256           string                 `json:"inputs_sha256"`
	SourceOrigin           CatalogRetentionOrigin `json:"source_origin"`
	SourceDescriptorSHA256 string                 `json:"source_descriptor_sha256"`
}

// CatalogRetentionInput supplies exact historical bytes without current deployment settings.
// Local descriptors use the original compressed localCatalogRecovery representation.
// The host validates fleet descriptors against their original publication inventory.
type CatalogRetentionInput struct {
	CatalogMaterializationInput
	SourceDescriptor []byte `json:"source_descriptor"`
}

// CatalogRetentionRequest retains one contiguous batch from a complete transfer manifest.
// All batches share TransferID and Manifest, and use distinct OperationID values.
// Writers must remain fenced. This operation never changes the active catalog inputs.
type CatalogRetentionRequest struct {
	Directory         string                  `json:"directory"`
	Owner             DirectoryOwner          `json:"owner"`
	SchedulerIdentity string                  `json:"scheduler_identity"`
	OperationID       string                  `json:"operation_id"`
	TransferID        string                  `json:"transfer_id"`
	Manifest          []CatalogRetentionEntry `json:"manifest"`
	BatchStart        int                     `json:"batch_start"`
	Inputs            []CatalogRetentionInput `json:"inputs"`
}

// CatalogRetainedRecord binds an immutable envelope to its manifest entry.
// Structural validation does not establish compatibility with current settings.
type CatalogRetainedRecord struct {
	Entry        CatalogRetentionEntry `json:"entry"`
	RecordSHA256 string                `json:"record_sha256"`
}

// CatalogRetentionReceipt records structural retention without selection or serving permission.
// It binds one native directory, the full manifest, and an exact batch range.
type CatalogRetentionReceipt struct {
	Version           int                       `json:"version"`
	Validation        string                    `json:"validation"`
	OperationID       string                    `json:"operation_id"`
	TransferID        string                    `json:"transfer_id"`
	RequestSHA256     string                    `json:"request_sha256"`
	PlanSHA256        string                    `json:"plan_sha256"`
	ManifestSHA256    string                    `json:"manifest_sha256"`
	BatchStart        int                       `json:"batch_start"`
	DirectoryIdentity privatefiles.EntryReceipt `json:"directory_identity"`
	Records           []CatalogRetainedRecord   `json:"records"`
}

type catalogRetentionPlan struct {
	Receipt CatalogRetentionReceipt `json:"receipt"`
}

// CatalogRetentionBatch selects an exact retained batch completion receipt.
type CatalogRetentionBatch struct {
	OperationID   string `json:"operation_id"`
	ReceiptSHA256 string `json:"receipt_sha256"`
}

// CatalogRetainedMaterializationRequest selects one entry from complete retained batch coverage.
// Batches must cover the full manifest once, in order, without gaps or overlaps.
type CatalogRetainedMaterializationRequest struct {
	Directory         string                  `json:"directory"`
	Owner             DirectoryOwner          `json:"owner"`
	SchedulerIdentity string                  `json:"scheduler_identity"`
	OperationID       string                  `json:"operation_id"`
	TransferID        string                  `json:"transfer_id"`
	Batches           []CatalogRetentionBatch `json:"batches"`
	Selected          int                     `json:"selected"`
}

// CatalogRetainedReadRequest selects one entry within an exact completed batch.
// Index is an absolute index in the complete transfer manifest.
type CatalogRetainedReadRequest struct {
	Directory         string                `json:"directory"`
	Owner             DirectoryOwner        `json:"owner"`
	SchedulerIdentity string                `json:"scheduler_identity"`
	TransferID        string                `json:"transfer_id"`
	Batch             CatalogRetentionBatch `json:"batch"`
	Index             int                   `json:"index"`
}

// ReadRetainedCatalogRecovery exports original historical bytes without selection or repair.
// It verifies the native owner, completed batch, manifest, and immutable envelope.
// Returned data belongs to the caller. Success confers no current permission.
func ReadRetainedCatalogRecovery(ctx context.Context, request CatalogRetainedReadRequest) (input CatalogRetentionInput, resultErr error) {
	directory, lock, reference, err := openRetainedCatalogRecord(ctx, request)
	if err != nil {
		return input, err
	}
	defer func() {
		resultErr = stderrors.Join(resultErr, lock.Close())
		if resultErr != nil {
			input = CatalogRetentionInput{}
		}
	}()
	input, err = readRetainedCatalogEnvelope(ctx, directory, reference)
	if err != nil {
		return CatalogRetentionInput{}, err
	}
	input.Generation = input.Generation.Copy()
	input.Recovery.Inputs.Data = bytes.Clone(input.Recovery.Inputs.Data)
	input.SourceDescriptor = bytes.Clone(input.SourceDescriptor)
	return input, ctx.Err()
}

// RetainCatalogRecovery preserves original historical capsules through a private operation journal.
// Validation is structural only. Pending retention blocks startup and selection.
// An exact completed retry returns its receipt without recreating missing records or changing later state.
func RetainCatalogRecovery(ctx context.Context, request CatalogRetentionRequest) (receipt CatalogRetentionReceipt, resultErr error) {
	if ctx == nil {
		return receipt, invalidInputPublication("retention requires a context")
	}
	requestData, records, err := validateCatalogRetentionRequest(ctx, request)
	if err != nil {
		return receipt, err
	}
	directory, lock, entry, store, err := openCatalogRecoveryOwner(ctx, materializationOwnerRequest(request))
	if err != nil {
		return receipt, err
	}
	defer func() {
		resultErr = stderrors.Join(resultErr, lock.Close())
		if resultErr != nil {
			receipt = CatalogRetentionReceipt{}
		}
	}()
	session, err := openCatalogRecoveryJournal(ctx, directory, store, materializationOwnerRequest(request), fleetRecoveryChecksum(requestData), entry, "", true, func() (*materializationPlan, error) {
		if err := retainCatalogTransferManifest(ctx, store, request.TransferID, request.Manifest); err != nil {
			return nil, err
		}
		plan, err := prepareCatalogRetentionPlan(ctx, directory, store, request, fleetRecoveryChecksum(requestData), entry, records)
		return plan, err
	})
	if err != nil {
		return receipt, err
	}
	if session.retainedComplete != nil {
		return *session.retainedComplete, nil
	}
	if session.complete != nil || session.plan.Retention == nil {
		return receipt, invalidInputPublication("operation belongs to catalog materialization")
	}
	plan := session.plan
	if err := checkCatalogTransferManifest(ctx, store, request.TransferID, request.Manifest); err != nil {
		return receipt, err
	}
	if err := checkMaterializationContinuity(ctx, directory, plan.Continuity); err != nil {
		return receipt, err
	}
	if err := checkCatalogRetentionSelection(ctx, store, plan); err != nil {
		return receipt, err
	}
	envelopes, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		return receipt, err
	}
	if err := envelopes.RecoverPublications(ctx); err != nil {
		return receipt, err
	}
	for i, input := range request.Inputs {
		data, err := encodeRetainedCatalogInput(request.Manifest[request.BatchStart+i], input)
		if err != nil {
			return receipt, err
		}
		reference := plan.Retention.Receipt.Records[i]
		if fleetRecoveryChecksum(data) != reference.RecordSHA256 {
			return receipt, invalidInputPublication("retention envelope changed")
		}
		previous, err := optionalMaterializationFile(envelopes, reference.RecordSHA256+".json.gz", maxRetainedCatalogRecordBytes)
		if err != nil {
			return receipt, err
		}
		if previous != nil && !bytes.Equal(previous, data) {
			return receipt, invalidInputPublication("immutable retention envelope changed")
		}
		if err := envelopes.CompareAndPublishFileContext(ctx, reference.RecordSHA256+".json.gz", previous, data, ".retained-input-"); err != nil {
			return receipt, err
		}
	}
	if err := checkMaterializationContinuity(ctx, directory, plan.Continuity); err != nil {
		return receipt, err
	}
	if err := checkCatalogRetentionSelection(ctx, store, plan); err != nil {
		return receipt, err
	}
	if err := inspectRuntimeInventory(ctx, directory); err != nil {
		return receipt, err
	}
	receipt = plan.Retention.Receipt
	receipt.PlanSHA256 = fleetRecoveryChecksum(session.encoded)
	data, err := json.Marshal(receipt, json.Deterministic(true))
	if err != nil {
		return CatalogRetentionReceipt{}, err
	}
	if err := session.directory.CompareAndPublishFileContext(ctx, materializationReceiptName, nil, data, ".materialization-complete-"); err != nil {
		return CatalogRetentionReceipt{}, err
	}
	return receipt, ctx.Err()
}

// MaterializeRetainedCatalogRecovery checks complete retained coverage and selects one capsule.
// Only the selected capsule must reproduce its generation under the supplied target settings.
// Completed retries return the original receipt without loading or recreating old envelopes.
func MaterializeRetainedCatalogRecovery(ctx context.Context, request CatalogRetainedMaterializationRequest, opts ...Option) (CatalogMaterializationReceipt, error) {
	binding, err := retainedMaterializationBinding(ctx, request, opts)
	if err != nil {
		return CatalogMaterializationReceipt{}, err
	}
	receipt, complete, err := completedRetainedMaterialization(ctx, request, binding)
	if err != nil || complete {
		return receipt, err
	}
	input, err := readRetainedCatalogSelection(ctx, request)
	if err != nil {
		return CatalogMaterializationReceipt{}, err
	}
	selected := CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: request.OperationID, Inputs: []CatalogMaterializationInput{input}}
	return materializeCatalogRecovery(ctx, selected, binding, opts...)
}

// InspectRetainedCatalogMaterialization verifies the current pre-activation input selection.
// It requires existing complete journals, exact batch coverage, and compatible target settings.
// A later input update refuses this check. Historical completed retries still return their original receipt.
// Inspection never repairs records, changes selection, or renews permission.
func InspectRetainedCatalogMaterialization(ctx context.Context, request CatalogRetainedMaterializationRequest, expected CatalogMaterializationReceipt, opts ...Option) (resultErr error) {
	binding, err := retainedMaterializationBinding(ctx, request, opts)
	if err != nil {
		return err
	}
	owner := CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: request.OperationID}
	directory, lock, entry, store, err := openCatalogRecoveryOwner(ctx, owner)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, lock.Close()) }()
	receipt, complete, err := completedRetainedMaterializationAt(ctx, request, binding, directory, entry, store)
	if err != nil {
		return err
	}
	if !complete || receipt != expected {
		return invalidInputPublication("current selection requires the exact completed materialization receipt")
	}
	current, err := checkMaterializationSelection(ctx, request.Directory)
	if err != nil {
		return err
	}
	if current == nil || *current != expected {
		return invalidInputPublication("current catalog selects another materialization operation")
	}
	input, err := readRetainedCatalogSelectionAt(ctx, request, directory, entry, store)
	if err != nil {
		return err
	}
	if err := ValidateCatalogReplay(ctx, input.Generation, input.Recovery, opts...); err != nil {
		return err
	}
	owner.Inputs = []CatalogMaterializationInput{input}
	if err := checkMaterializedInputs(ctx, store, owner); err != nil {
		return err
	}
	return inspectRuntimeInventory(ctx, directory)
}

func materializationOwnerRequest(request CatalogRetentionRequest) CatalogMaterializationRequest {
	return CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: request.OperationID}
}

func openCatalogRecoveryOwner(ctx context.Context, request CatalogMaterializationRequest) (*privatefiles.Directory, *flock.Flock, privatefiles.EntryReceipt, *layerStore, error) {
	directory, err := privatefiles.ExistingDirectory(request.Directory)
	if err != nil {
		return nil, nil, privatefiles.EntryReceipt{}, nil, err
	}
	lock, err := acquireDirectory(ctx, request.Directory)
	if err != nil {
		return nil, nil, privatefiles.EntryReceipt{}, nil, err
	}
	fail := func(err error) (*privatefiles.Directory, *flock.Flock, privatefiles.EntryReceipt, *layerStore, error) {
		return nil, nil, privatefiles.EntryReceipt{}, nil, stderrors.Join(err, lock.Close())
	}
	root, err := directory.Open()
	if err != nil {
		return fail(err)
	}
	entry, err := privatefiles.CaptureEntry(root, ".", true)
	if err = stderrors.Join(err, root.Close()); err != nil {
		return fail(err)
	}
	if err := inspectMaterializationOwner(ctx, directory, request); err != nil {
		return fail(err)
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil || !store.durable() {
		return fail(stderrors.Join(err, invalidInputPublication("recovery requires retained catalog state")))
	}
	return directory, lock, entry, store, nil
}

func prepareCatalogRetentionPlan(ctx context.Context, directory *privatefiles.Directory, store *layerStore, request CatalogRetentionRequest, requestSHA string, entry privatefiles.EntryReceipt, records []CatalogRetainedRecord) (*materializationPlan, error) {
	manifest, err := catalogRetentionManifestBytes(request.TransferID, request.Manifest)
	if err != nil {
		return nil, err
	}
	plan := &materializationPlan{Retention: &catalogRetentionPlan{Receipt: CatalogRetentionReceipt{Version: materializationVersion, Validation: "structural", OperationID: request.OperationID, TransferID: request.TransferID, RequestSHA256: requestSHA, ManifestSHA256: fleetRecoveryChecksum(manifest), BatchStart: request.BatchStart, DirectoryIdentity: entry, Records: records}}}
	plan.Continuity, err = captureMaterializationContinuity(ctx, directory)
	if err != nil {
		return nil, err
	}
	active, err := captureMaterializationFiles(ctx, store.directory)
	if err != nil {
		return nil, err
	}
	for _, name := range sortedMaterializationNames(active) {
		plan.Changes = append(plan.Changes, materializationChange{Name: name, Before: active[name], After: active[name]})
	}
	plan.PreviousSelection, err = optionalMaterializationFile(store.directory, materializationSelectionName, maxLayerBytes)
	return plan, err
}

func checkCatalogRetentionSelection(ctx context.Context, store *layerStore, plan *materializationPlan) error {
	active, err := captureMaterializationFiles(ctx, store.directory)
	if err != nil {
		return err
	}
	expected := map[string][]byte{}
	for _, change := range plan.Changes {
		expected[change.Name] = change.Before
	}
	if !reflect.DeepEqual(active, expected) {
		return invalidInputPublication("active catalog changed during historical retention")
	}
	selection, err := optionalMaterializationFile(store.directory, materializationSelectionName, maxLayerBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(selection, plan.PreviousSelection) {
		return invalidInputPublication("catalog selection changed during historical retention")
	}
	return ctx.Err()
}

func readCatalogRetentionReceipt(directory *privatefiles.Directory, plan materializationPlan, encoded []byte) (*CatalogRetentionReceipt, error) {
	raw, err := optionalMaterializationFile(directory, materializationReceiptName, maxLayerBytes)
	if err != nil || raw == nil {
		return nil, err
	}
	var receipt CatalogRetentionReceipt
	if err := json.Unmarshal(raw, &receipt, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	expected := plan.Retention.Receipt
	expected.PlanSHA256 = fleetRecoveryChecksum(encoded)
	if !reflect.DeepEqual(receipt, expected) {
		return nil, invalidInputPublication("retention completion differs from its journal")
	}
	return &receipt, nil
}

func validateCatalogRetentionPlan(plan materializationPlan) error {
	if plan.Retention == nil || plan.Receipt != (CatalogMaterializationReceipt{}) || plan.RetainedBindingSHA256 != "" {
		return invalidInputPublication("retention journal contains selection authority")
	}
	r := plan.Retention.Receipt
	if r.Version != materializationVersion || r.Validation != "structural" || !validRecoveryOperationID(r.OperationID) || !validRecoveryOperationID(r.TransferID) || !validFleetChecksum(r.RequestSHA256) || !validFleetChecksum(r.ManifestSHA256) || r.PlanSHA256 != "" || r.DirectoryIdentity.Identity == "" || r.DirectoryIdentity.Access == "" || r.BatchStart < 0 || len(r.Records) == 0 || len(r.Records) > storage.DefaultRetentionScanEntries || r.BatchStart > storage.DefaultRetentionScanEntries-len(r.Records) || len(plan.Changes) > storage.DefaultRetentionScanEntries {
		return invalidInputPublication("retention journal has invalid identity or bounds")
	}
	for _, record := range r.Records {
		if validateCatalogRetentionEntry(record.Entry) != nil || !validFleetChecksum(record.RecordSHA256) {
			return invalidInputPublication("retention journal has invalid record")
		}
	}
	prior := ""
	for _, change := range plan.Changes {
		if !managedMaterializationFile(change.Name) || change.Name <= prior || len(change.Before) > maxLayerBytes || !bytes.Equal(change.Before, change.After) {
			return invalidInputPublication("retention journal changes active catalog inputs")
		}
		prior = change.Name
	}
	return nil
}

func openRetainedCatalogRecord(ctx context.Context, request CatalogRetainedReadRequest) (transfer *privatefiles.Directory, retainedLock *flock.Flock, reference CatalogRetainedRecord, resultErr error) {
	if ctx == nil || !filepath.IsAbs(request.Directory) || filepath.Clean(request.Directory) != request.Directory || !validRecoveryOperationID(request.TransferID) || !validRecoveryOperationID(request.Batch.OperationID) || !validFleetChecksum(request.Batch.ReceiptSHA256) || request.Index < 0 || request.Index >= storage.DefaultRetentionScanEntries {
		return nil, nil, CatalogRetainedRecord{}, invalidInputPublication("retained read requires an exact bounded entry reference")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	if err := request.Owner.Validate(); err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	owner := CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity}
	directoryOwner, lock, entry, store, err := openCatalogRecoveryOwner(ctx, owner)
	if err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = stderrors.Join(resultErr, lock.Close())
		}
	}()

	receipt, err := readRetainedBatch(ctx, directoryOwner, store, request.Batch)
	if err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	if receipt.TransferID != request.TransferID || receipt.DirectoryIdentity != entry || request.Index < receipt.BatchStart || request.Index-receipt.BatchStart >= len(receipt.Records) {
		return nil, nil, CatalogRetainedRecord{}, invalidInputPublication("retained read differs from its batch scope or native owner")
	}
	directory, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	raw, err := directory.ReadFile("manifest.json", maxLayerBytes)
	if err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	var manifest retainedCatalogManifest
	if err := json.Unmarshal(raw, &manifest, json.RejectUnknownMembers(true)); err != nil {
		return nil, nil, CatalogRetainedRecord{}, err
	}
	canonical, err := catalogRetentionManifestBytes(request.TransferID, manifest.Entries)
	if err != nil || manifest.TransferID != request.TransferID || manifest.Version != materializationVersion || !bytes.Equal(raw, canonical) || fleetRecoveryChecksum(raw) != receipt.ManifestSHA256 || request.Index >= len(manifest.Entries) {
		return nil, nil, CatalogRetainedRecord{}, invalidInputPublication("retained read manifest changed")
	}
	reference = receipt.Records[request.Index-receipt.BatchStart]
	if reference.Entry != manifest.Entries[request.Index] {
		return nil, nil, CatalogRetainedRecord{}, invalidInputPublication("retained read entry changed")
	}
	return directory, lock, reference, nil
}
