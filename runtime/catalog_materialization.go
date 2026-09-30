package runtime

import (
	"bytes"
	"context"
	"encoding/hex"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const (
	materializationsDirectory    = "materializations"
	materializationSelectionName = "materialization.json"
	materializationPlanName      = "plan.json.gz"
	materializationReceiptName   = "complete.json"
	recoveryBaselineName         = "recovery-baseline.json"
	materializationVersion       = 1
)

// CatalogMaterializationInput binds one retained generation to its exact reconstruction inputs.
// The caller retains catalog-store selection and authority evidence separately.
type CatalogMaterializationInput struct {
	Generation catalogs.Generation `json:"generation"`
	Recovery   CatalogRecovery     `json:"recovery"`
}

// CatalogMaterializationRequest selects private catalog inputs for an existing stopped runtime.
// Selected is an index into Inputs. Each input must reproduce its generation under the supplied options.
// Writers must remain fenced throughout recovery. This API grants no publication or serving permission.
type CatalogMaterializationRequest struct {
	Directory         string                        `json:"directory"`
	Owner             DirectoryOwner                `json:"owner"`
	SchedulerIdentity string                        `json:"scheduler_identity"`
	OperationID       string                        `json:"operation_id"`
	Selected          int                           `json:"selected"`
	Inputs            []CatalogMaterializationInput `json:"inputs"`
}

// CatalogMaterializationReceipt records completed private-file replacement without catalog-store activation.
// DirectoryIdentity binds native ownership. InventorySHA256 binds the supplied ordered historical inputs.
type CatalogMaterializationReceipt struct {
	Version                int                       `json:"version"`
	OperationID            string                    `json:"operation_id"`
	RequestSHA256          string                    `json:"request_sha256"`
	PlanSHA256             string                    `json:"plan_sha256"`
	InventorySHA256        string                    `json:"inventory_sha256"`
	DirectoryIdentity      privatefiles.EntryReceipt `json:"directory_identity"`
	SelectedManifestSHA256 string                    `json:"selected_manifest_sha256"`
	SelectedInputsSHA256   string                    `json:"selected_inputs_sha256"`
	CatalogPublisherID     string                    `json:"catalog_publisher_id"`
	BaselineManifestSHA256 string                    `json:"baseline_manifest_sha256"`
}

type materializationChange struct {
	Name   string `json:"name"`
	Before []byte `json:"before"`
	After  []byte `json:"after"`
}

type materializationPlan struct {
	Receipt               CatalogMaterializationReceipt         `json:"receipt"`
	Continuity            map[string]privatefiles.RecordReceipt `json:"continuity"`
	Changes               []materializationChange               `json:"changes"`
	PreviousSelection     []byte                                `json:"previous_selection"`
	Retention             *catalogRetentionPlan                 `json:"retention,omitempty"`
	RetainedBindingSHA256 string                                `json:"retained_binding_sha256,omitempty"`
}

type materializationSelection struct {
	Version         int    `json:"version"`
	OperationSHA256 string `json:"operation_sha256"`
}
type recoveryBaselineRecord struct {
	PublisherID    string `json:"publisher_id"`
	Version        int    `json:"version"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

// MaterializeCatalogRecovery replaces catalog inputs through a private operation journal.
// It preserves owner, instance seed, permission checkpoints, and origin discovery records.
// Pending work blocks runtime startup. An exact completed retry returns its original receipt without replacing later state.
func MaterializeCatalogRecovery(ctx context.Context, request CatalogMaterializationRequest, opts ...Option) (receipt CatalogMaterializationReceipt, resultErr error) {
	return materializeCatalogRecovery(ctx, request, "", opts...)
}

func materializeCatalogRecovery(ctx context.Context, request CatalogMaterializationRequest, retainedBinding string, opts ...Option) (receipt CatalogMaterializationReceipt, resultErr error) {
	if ctx == nil {
		return receipt, invalidInputPublication("materialization requires a context")
	}
	if err := ctx.Err(); err != nil {
		return receipt, err
	}
	requestData, err := validateMaterializationRequest(ctx, request, opts)
	if err != nil {
		return receipt, err
	}
	directory, lock, entry, store, err := openCatalogRecoveryOwner(ctx, request)
	if err != nil {
		return receipt, err
	}
	defer func() {
		resultErr = stderrors.Join(resultErr, lock.Close())
		if resultErr != nil {
			receipt = CatalogMaterializationReceipt{}
		}
	}()
	session, err := openCatalogRecoveryJournal(ctx, directory, store, request, fleetRecoveryChecksum(requestData), entry, retainedBinding, false, func() (*materializationPlan, error) {
		plan, err := prepareMaterializationPlan(ctx, store, request, fleetRecoveryChecksum(requestData), entry)
		if plan != nil {
			plan.RetainedBindingSHA256 = retainedBinding
		}
		return plan, err
	})
	if err != nil {
		return receipt, err
	}
	if session.retainedComplete != nil || session.plan != nil && session.plan.Retention != nil {
		return receipt, invalidInputPublication("operation belongs to historical retention")
	}
	if session.complete != nil {
		return *session.complete, nil
	}
	plan, encoded, journal := session.plan, session.encoded, session.directory
	if err := checkMaterializationContinuity(ctx, directory, plan.Continuity); err != nil {
		return receipt, err
	}
	// Retain each historical capsule before replacing active inputs. Native records remain immutable.
	for _, input := range request.Inputs {
		if err := retainMaterializationInput(ctx, store, input); err != nil {
			return receipt, err
		}
	}
	if err := applyMaterializationChanges(ctx, store.directory, plan.Changes); err != nil {
		return receipt, err
	}
	if err := checkMaterializationContinuity(ctx, directory, plan.Continuity); err != nil {
		return receipt, err
	}
	if err := checkMaterializedInputs(ctx, store, request); err != nil {
		return receipt, err
	}
	if err := inspectRuntimeInventory(ctx, directory); err != nil {
		return receipt, err
	}
	receipt = plan.Receipt
	receipt.PlanSHA256 = fleetRecoveryChecksum(encoded)
	data, err := json.Marshal(receipt, json.Deterministic(true))
	if err != nil {
		return CatalogMaterializationReceipt{}, err
	}
	if err := journal.CompareAndPublishFileContext(ctx, materializationReceiptName, nil, data, ".materialization-complete-"); err != nil {
		return CatalogMaterializationReceipt{}, err
	}
	return receipt, ctx.Err()
}

func validateMaterializationRequest(ctx context.Context, request CatalogMaterializationRequest, opts []Option) ([]byte, error) {
	if !filepath.IsAbs(request.Directory) || filepath.Clean(request.Directory) != request.Directory || strings.TrimSpace(request.OperationID) != request.OperationID || request.OperationID == "" || len(request.OperationID) > deploymentIDMaxBytes || !utf8.ValidString(request.OperationID) || strings.ContainsFunc(request.OperationID, unicode.IsControl) {
		return nil, invalidInputPublication("materialization requires an absolute directory and bounded operation identity")
	}
	if err := request.Owner.Validate(); err != nil {
		return nil, err
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > storage.DefaultRetentionScanEntries || request.Selected < 0 || request.Selected >= len(request.Inputs) {
		return nil, invalidInputPublication("materialization requires a selected bounded input inventory")
	}
	seen := make(map[string]bool)
	var total, decodedTotal int64
	for _, input := range request.Inputs {
		total += int64(len(input.Generation.Payload) + len(input.Recovery.Inputs.Data))
		if total > storage.DefaultRetentionInputMaxBytes {
			return nil, invalidInputPublication("materialization input inventory exceeds its byte limit")
		}
		identity := input.Recovery.ManifestChecksum + ":" + input.Recovery.Inputs.Checksum
		if seen[identity] {
			return nil, invalidInputPublication("materialization contains repeated generation inputs")
		}
		seen[identity] = true
		decoded, err := decompressFleetRecovery(ctx, input.Recovery.Inputs.Data, storage.DefaultRetentionInputMaxBytes-decodedTotal)
		if err != nil {
			return nil, err
		}
		decodedTotal += int64(len(decoded))
		if err := ValidateCatalogReplay(ctx, input.Generation, input.Recovery, opts...); err != nil {
			return nil, err
		}
	}
	return json.Marshal(request, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
}

func inspectMaterializationOwner(ctx context.Context, directory *privatefiles.Directory, request CatalogMaterializationRequest) error {
	expected, err := encodeOwnerRecord(request.Owner, request.SchedulerIdentity)
	if err != nil {
		return err
	}
	actual, err := directory.ReadFile(ownerRecordName, ownerRecordMaxBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return invalidInputPublication("materialization directory belongs to another owner")
	}
	seed, err := directory.ReadFile(instanceSeedFileName, instanceSeedBytes*2)
	if err != nil {
		return err
	}
	decoded, decodeErr := hex.DecodeString(string(seed))
	if decodeErr != nil || len(decoded) != instanceSeedBytes || hex.EncodeToString(decoded) != string(seed) {
		return invalidInputPublication("materialization requires the original instance seed")
	}
	return ctx.Err()
}

func retainMaterializationInput(ctx context.Context, store *layerStore, input CatalogMaterializationInput) error {
	record, err := readFleetRecovery(ctx, input.Recovery.Inputs.Data)
	if err != nil {
		return err
	}
	baseline, err := encodeCatalogRecoveryBaseline(record.Baseline)
	if err != nil {
		return err
	}
	record.Baseline = catalogs.Generation{}
	local := localCatalogRecovery{Version: localCatalogRecoveryVersion, ManifestChecksum: input.Recovery.ManifestChecksum, GenerationID: input.Generation.Manifest.GenerationID, PayloadChecksum: input.Generation.Manifest.Payload.Checksum, BaselineChecksum: baseline.manifestChecksum, Inputs: record}
	if err := local.validate(input.Generation); err != nil {
		return err
	}
	return store.retainCatalogRecovery(ctx, local, baseline)
}

func optionalMaterializationFile(directory *privatefiles.Directory, name string, limit int64) ([]byte, error) {
	data, err := directory.ReadFile(name, limit)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func materializationFileDirectory(root *privatefiles.Directory, name string, create bool) (*privatefiles.Directory, string, error) {
	if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
		return nil, "", invalidInputPublication("materialization has an unsafe file reference")
	}
	parts := strings.Split(name, "/")
	directory := root
	for _, part := range parts[:len(parts)-1] {
		var err error
		if create {
			directory, err = directory.Child(part)
		} else {
			directory, err = directory.ExistingChild(part)
		}
		if err != nil {
			return nil, "", err
		}
	}
	return directory, parts[len(parts)-1], nil
}

func applyMaterializationChanges(ctx context.Context, root *privatefiles.Directory, changes []materializationChange) error {
	for _, change := range changes {
		directory, name, err := materializationFileDirectory(root, change.Name, true)
		if err != nil {
			return err
		}
		actual, err := optionalMaterializationFile(directory, name, maxLayerBytes)
		if err != nil {
			return err
		}
		if bytes.Equal(actual, change.After) {
			continue
		}
		if !bytes.Equal(actual, change.Before) {
			return invalidInputPublication("catalog inputs changed during materialization")
		}
		if change.After == nil {
			if _, err := directory.CompareAndRemoveFileContext(ctx, name, actual); err != nil {
				return err
			}
		} else if err := directory.CompareAndPublishFileContext(ctx, name, actual, change.After, ".layer-"); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func encodeMaterializationPlan(plan materializationPlan) ([]byte, error) {
	data, err := json.Marshal(plan, json.Deterministic(true), json.FormatNilSliceAsNull(true))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFleetRecoveryBytes {
		return nil, invalidInputPublication("materialization journal exceeds its byte limit")
	}
	return compressFleetRecovery(data)
}

func readMaterializationPlan(ctx context.Context, directory *privatefiles.Directory) (*materializationPlan, []byte, error) {
	raw, err := optionalMaterializationFile(directory, materializationPlanName, MaxFleetRecoveryBytes)
	if err != nil || raw == nil {
		return nil, nil, err
	}
	data, err := decompressFleetRecovery(ctx, raw, MaxFleetRecoveryBytes)
	if err != nil {
		return nil, nil, err
	}
	var plan materializationPlan
	if err := json.Unmarshal(data, &plan, json.RejectUnknownMembers(true)); err != nil {
		return nil, nil, err
	}
	if plan.Retention != nil {
		if err := validateCatalogRetentionPlan(plan); err != nil {
			return nil, nil, err
		}
		return &plan, raw, ctx.Err()
	}
	receipt := plan.Receipt
	if plan.RetainedBindingSHA256 != "" && !validFleetChecksum(plan.RetainedBindingSHA256) {
		return nil, nil, invalidInputPublication("materialization has invalid retained binding")
	}
	if receipt.Version != materializationVersion || receipt.OperationID == "" || !validFleetChecksum(receipt.RequestSHA256) || !validFleetChecksum(receipt.InventorySHA256) || !validFleetChecksum(receipt.SelectedManifestSHA256) || !validFleetChecksum(receipt.SelectedInputsSHA256) || !validFleetChecksum(receipt.BaselineManifestSHA256) || receipt.CatalogPublisherID == "" || receipt.PlanSHA256 != "" || receipt.DirectoryIdentity.Identity == "" || receipt.DirectoryIdentity.Access == "" || len(plan.Changes) > storage.DefaultRetentionScanEntries {
		return nil, nil, invalidInputPublication("materialization journal has invalid identity or bounds")
	}
	prior := ""
	for _, change := range plan.Changes {
		if !managedMaterializationFile(change.Name) || change.Name <= prior || len(change.Before) > maxLayerBytes || len(change.After) > maxLayerBytes {
			return nil, nil, invalidInputPublication("materialization journal has invalid catalog changes")
		}
		prior = change.Name
	}
	return &plan, raw, ctx.Err()
}

func readMaterializationReceipt(directory *privatefiles.Directory, plan materializationPlan, encoded []byte) (*CatalogMaterializationReceipt, error) {
	raw, err := optionalMaterializationFile(directory, materializationReceiptName, maxLayerBytes)
	if err != nil || raw == nil {
		return nil, err
	}
	var receipt CatalogMaterializationReceipt
	if err := json.Unmarshal(raw, &receipt, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	expected := plan.Receipt
	expected.PlanSHA256 = fleetRecoveryChecksum(encoded)
	if receipt != expected {
		return nil, invalidInputPublication("materialization completion differs from its immutable journal")
	}
	return &receipt, nil
}

func refusePendingCatalogMaterialization(ctx context.Context, directory string) error {
	receipt, err := checkMaterializationSelection(ctx, directory)
	if err != nil || receipt == nil {
		return err
	}
	root, err := privatefiles.ExistingDirectory(directory)
	if err != nil {
		return err
	}
	opened, err := root.Open()
	if err != nil {
		return err
	}
	err = privatefiles.CheckEntry(opened, ".", true, receipt.DirectoryIdentity)
	return stderrors.Join(err, opened.Close(), ctx.Err())
}
func checkMaterializationSelection(ctx context.Context, directory string) (*CatalogMaterializationReceipt, error) {
	if directory == "" {
		return nil, nil
	}
	store, err := existingLayerStore(directory)
	if err != nil || !store.durable() {
		return nil, err
	}
	if err := inspectMaterializationJournals(ctx, store); err != nil {
		return nil, err
	}
	selection, err := optionalMaterializationFile(store.directory, materializationSelectionName, maxLayerBytes)
	if err != nil {
		return nil, err
	}
	if selection == nil {
		checkpoint, err := optionalMaterializationFile(store.directory, recoveryBaselineName, maxLayerBytes)
		if err != nil {
			return nil, err
		}
		if checkpoint != nil {
			return nil, invalidInputPublication("explicit recovery baseline has no selected materialization")
		}
		return nil, ctx.Err()
	}
	var record materializationSelection
	if err := json.Unmarshal(selection, &record, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if record.Version != materializationVersion || !validFleetChecksum(record.OperationSHA256) {
		return nil, invalidInputPublication("materialization selection is invalid")
	}
	parent, err := store.directory.ExistingChild(materializationsDirectory)
	if err != nil {
		return nil, err
	}
	journal, err := parent.ExistingChild(record.OperationSHA256)
	if err != nil {
		return nil, err
	}
	plan, encoded, err := readMaterializationPlan(ctx, journal)
	if err != nil {
		return nil, err
	}
	if plan == nil || fleetRecoveryChecksum([]byte(plan.Receipt.OperationID)) != record.OperationSHA256 {
		return nil, invalidInputPublication("materialization selection has no matching journal")
	}
	receipt, err := readMaterializationReceipt(journal, *plan, encoded)
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, invalidInputPublication("complete catalog materialization before runtime startup")
	}
	checkpoint, err := store.loadRecoveryBaseline(ctx)
	if err != nil {
		return nil, err
	}
	if checkpoint == nil || checkpoint.ManifestSHA256 != receipt.BaselineManifestSHA256 || checkpoint.PublisherID != receipt.CatalogPublisherID {
		return nil, invalidInputPublication("explicit recovery baseline differs from materialization receipt")
	}
	return receipt, ctx.Err()
}

type materializationSession struct {
	plan             *materializationPlan
	encoded          []byte
	directory        *privatefiles.Directory
	complete         *CatalogMaterializationReceipt
	retainedComplete *CatalogRetentionReceipt
}

func openCatalogRecoveryJournal(ctx context.Context, directory *privatefiles.Directory, store *layerStore, request CatalogMaterializationRequest, requestSHA string, entry privatefiles.EntryReceipt, retainedBinding string, retention bool, prepare func() (*materializationPlan, error)) (*materializationSession, error) {
	// Recover only owner-owned file publication journals. No source, catalog store, or authority operation runs here.
	if err := store.recoverRecordPublications(ctx); err != nil {
		return nil, err
	}
	operation := fleetRecoveryChecksum([]byte(request.OperationID))
	journals, err := store.directory.Child(materializationsDirectory)
	if err != nil {
		return nil, err
	}
	journal, err := journals.Child(operation)
	if err != nil {
		return nil, err
	}
	if err := journal.RecoverPublications(ctx); err != nil {
		return nil, err
	}
	plan, encoded, err := readMaterializationPlan(ctx, journal)
	if err != nil {
		return nil, err
	}
	if plan != nil {
		session, err := resumeCatalogRecoveryJournal(directory, journal, plan, encoded, requestSHA, entry, retainedBinding, retention)
		if err != nil || session != nil {
			return session, err
		}
	} else {
		if _, err := checkMaterializationSelection(ctx, request.Directory); err != nil {
			return nil, err
		}
		if err := InspectRetainedDirectory(ctx, request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
			return nil, err
		}
		if err := store.refuseInputPublication(); err != nil {
			return nil, err
		}
		plan, err = prepare()
		if err != nil {
			return nil, err
		}
		encoded, err = encodeMaterializationPlan(*plan)
		if err != nil {
			return nil, err
		}
		if err := journal.CompareAndPublishFileContext(ctx, materializationPlanName, nil, encoded, ".materialization-plan-"); err != nil {
			return nil, err
		}
	}
	if plan.Retention != nil {
		return &materializationSession{plan: plan, encoded: encoded, directory: journal}, nil
	}
	selection, err := json.Marshal(materializationSelection{Version: materializationVersion, OperationSHA256: operation}, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	actual, err := optionalMaterializationFile(store.directory, materializationSelectionName, maxLayerBytes)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(actual, selection) {
		if !bytes.Equal(actual, plan.PreviousSelection) {
			return nil, invalidInputPublication("another materialization selected this directory")
		}
		if err := store.directory.CompareAndPublishFileContext(ctx, materializationSelectionName, actual, selection, ".materialization-selection-"); err != nil {
			return nil, err
		}
	}
	return &materializationSession{plan: plan, encoded: encoded, directory: journal}, nil
}

func resumeCatalogRecoveryJournal(directory *privatefiles.Directory, journal *privatefiles.Directory, plan *materializationPlan, encoded []byte, requestSHA string, entry privatefiles.EntryReceipt, retainedBinding string, retention bool) (*materializationSession, error) {
	if (plan.Retention != nil) != retention || plan.RetainedBindingSHA256 != retainedBinding {
		return nil, invalidInputPublication("catalog recovery operation kind or retained binding changed")
	}
	if plan.Retention != nil {
		if plan.Retention.Receipt.RequestSHA256 != requestSHA || plan.Retention.Receipt.DirectoryIdentity != entry {
			return nil, invalidInputPublication("retention operation or native directory changed")
		}
		complete, err := readCatalogRetentionReceipt(journal, *plan, encoded)
		if err != nil {
			return nil, err
		}
		if complete != nil {
			if err := checkMaterializationIdentity(directory, plan.Continuity); err != nil {
				return nil, err
			}
			return &materializationSession{retainedComplete: complete}, nil
		}
	} else if plan.Receipt.RequestSHA256 != requestSHA || plan.Receipt.DirectoryIdentity != entry {
		return nil, invalidInputPublication("materialization operation or native directory changed")
	}
	if plan.Retention == nil {
		if complete, err := readMaterializationReceipt(journal, *plan, encoded); err != nil || complete != nil {
			if complete != nil {
				if identityErr := checkMaterializationIdentity(directory, plan.Continuity); identityErr != nil {
					return nil, identityErr
				}
				return &materializationSession{complete: complete}, err
			}
			return nil, err
		}
	}

	return nil, nil
}
