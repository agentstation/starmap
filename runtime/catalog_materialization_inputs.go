package runtime

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	stderrors "errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func prepareMaterializationPlan(ctx context.Context, store *layerStore, request CatalogMaterializationRequest, requestSHA string, entry privatefiles.EntryReceipt) (*materializationPlan, error) {
	selected := request.Inputs[request.Selected]
	record, err := readFleetRecovery(ctx, selected.Recovery.Inputs.Data)
	if err != nil {
		return nil, err
	}
	baseline, err := catalogManifestChecksum(record.Baseline)
	if err != nil {
		return nil, err
	}
	inventory := make([][2]string, 0, len(request.Inputs))
	for _, input := range request.Inputs {
		inventory = append(inventory, [2]string{input.Recovery.ManifestChecksum, input.Recovery.Inputs.Checksum})
	}
	inventoryData, err := json.Marshal(inventory, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	desired, err := materializationInputs(record, baseline)
	if err != nil {
		return nil, err
	}
	plan := &materializationPlan{Receipt: CatalogMaterializationReceipt{Version: materializationVersion, OperationID: request.OperationID, RequestSHA256: requestSHA, InventorySHA256: fleetRecoveryChecksum(inventoryData), DirectoryIdentity: entry, SelectedManifestSHA256: selected.Recovery.ManifestChecksum, SelectedInputsSHA256: selected.Recovery.Inputs.Checksum, CatalogPublisherID: record.PublisherID, BaselineManifestSHA256: baseline}}
	previous, err := captureMaterializationFiles(ctx, store.directory)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool)
	for name := range previous {
		names[name] = true
	}
	for name := range desired {
		names[name] = true
	}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		plan.Changes = append(plan.Changes, materializationChange{Name: name, Before: previous[name], After: desired[name]})
	}
	parent, err := privatefiles.ExistingDirectory(request.Directory)
	if err != nil {
		return nil, err
	}
	plan.Continuity, err = captureMaterializationContinuity(ctx, parent)
	if err != nil {
		return nil, err
	}
	plan.PreviousSelection, err = optionalMaterializationFile(store.directory, materializationSelectionName, maxLayerBytes)
	return plan, err
}

func materializationInputs(record fleetRecoveryRecord, baseline string) (map[string][]byte, error) {
	inputs := make(map[string][]byte)
	add := func(name string, value any) error {
		data, err := jsonv1.Marshal(value)
		if err != nil {
			return err
		}
		if len(data) > maxLayerBytes {
			return invalidInputPublication("materialization input exceeds its record limit")
		}
		inputs[name] = data
		return nil
	}
	if err := add(recoveryBaselineName, recoveryBaselineRecord{Version: materializationVersion, ManifestSHA256: baseline, PublisherID: record.PublisherID}); err != nil {
		return nil, err
	}
	if err := add(inputPublicationName, inputPublication{Version: inputPublicationVersion, Phase: inputPublicationIdle}); err != nil {
		return nil, err
	}
	if record.Source != nil {
		if err := add(sourceLayerFileName, record.Source); err != nil {
			return nil, err
		}
	}
	for _, layer := range record.Providers {
		directory := providerLayerDirectoryName
		if layer.evidenceKey().scoped() {
			directory += "/" + bindingLayerDirectoryName
		}
		if err := add(directory+"/"+layer.evidenceKey().filename(), layer); err != nil {
			return nil, err
		}
	}
	if record.Manual != nil {
		checkpoint, err := jsonv1.Marshal(manualBatchRecord{Version: manualHistoryVersion, Checkpoint: record.Manual})
		if err != nil {
			return nil, err
		}
		reference := fleetRecoveryChecksum(checkpoint) + ".json"
		inputs[inputPublicationDirectory+"/"+reference] = checkpoint
		if err := add(manualHistoryName, manualHistoryHead{Version: manualHistoryVersion, Batch: reference}); err != nil {
			return nil, err
		}
	}
	if record.Removals != nil {
		if err := add(removalPolicyName, removalPolicyRecord{Version: removalPolicyVersion, Policy: *record.Removals}); err != nil {
			return nil, err
		}
	}
	if record.Pin != nil {
		if err := add(generationPinRecordFile, record.Pin); err != nil {
			return nil, err
		}
	}
	return inputs, nil
}

func managedMaterializationFile(name string) bool {
	switch name {
	case sourceLayerFileName, manualHistoryName, removalPolicyName, inputPublicationName, generationPinRecordFile, recoveryBaselineName:
		return true
	}
	parent, base := path.Dir(name), path.Base(name)
	if parent == inputPublicationDirectory {
		return validInputReference(base)
	}
	return (parent == providerLayerDirectoryName || parent == providerLayerDirectoryName+"/"+bindingLayerDirectoryName) && strings.HasSuffix(base, ".json")
}

func captureMaterializationFiles(ctx context.Context, directory *privatefiles.Directory) (files map[string][]byte, resultErr error) {
	files = make(map[string][]byte)
	root, err := directory.Open()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	var total int64
	err = walkMigrationTree(ctx, root, storage.DefaultRetentionScanEntries, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Historical immutable publication inputs remain intact. Only the selected head changes.
		if name == "." || entry.IsDir() || !managedMaterializationFile(name) || path.Dir(name) == inputPublicationDirectory {
			return nil
		}
		parent, base, err := materializationFileDirectory(directory, name, false)
		if err != nil {
			return err
		}
		data, err := parent.ReadFile(base, min(int64(maxLayerBytes), storage.DefaultRetentionInputMaxBytes-total))
		if err != nil {
			return err
		}
		total += int64(len(data))
		files[name] = data
		return nil
	})
	return files, err
}

func captureMaterializationContinuity(ctx context.Context, directory *privatefiles.Directory) (records map[string]privatefiles.RecordReceipt, resultErr error) {
	records = make(map[string]privatefiles.RecordReceipt)
	root, err := directory.Open()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	err = walkMigrationTree(ctx, root, migrationSourceMaxEntries, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." || entry.IsDir() || name == directoryLockName {
			return nil
		}
		if strings.HasPrefix(name, layerDirectoryName+"/") {
			relative := strings.TrimPrefix(name, layerDirectoryName+"/")
			// Permission continuity is independent of catalog facts. Other catalog files have their own journal or immutable owner.
			if relative != permissionCheckpointFile {
				return nil
			}
		}
		if isRecordPublicationDirectory(path.Dir(name)) {
			return nil
		}
		parent, base, err := materializationFileDirectory(directory, name, false)
		if err != nil {
			return err
		}
		opened, err := parent.Open()
		if err != nil {
			return err
		}
		limit, _, err := runtimeRecoveryFileLimit(name)
		if err != nil {
			_ = opened.Close()
			return err
		}
		receipt, err := privatefiles.CaptureRecord(opened, base, limit)
		closeErr := opened.Close()
		if err != nil || closeErr != nil {
			return stderrors.Join(err, closeErr)
		}
		records[name] = receipt
		return nil
	})
	return records, err
}

func checkMaterializationContinuity(ctx context.Context, directory *privatefiles.Directory, records map[string]privatefiles.RecordReceipt) error {
	actual, err := captureMaterializationContinuity(ctx, directory)
	if err != nil {
		return err
	}
	if len(actual) != len(records) {
		return invalidInputPublication("runtime authority or identity continuity changed")
	}
	for name, expected := range records {
		if actual[name] != expected {
			return invalidInputPublication("runtime authority or identity continuity changed")
		}
	}
	return nil
}

func (s *layerStore) loadRecoveryBaseline(ctx context.Context) (*recoveryBaselineRecord, error) {
	if !s.durable() {
		return nil, nil
	}
	data, err := optionalMaterializationFile(s.directory, recoveryBaselineName, maxLayerBytes)
	if err != nil || data == nil {
		return nil, err
	}
	var record recoveryBaselineRecord
	if err := json.Unmarshal(data, &record, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if record.Version != materializationVersion || !validFleetChecksum(record.ManifestSHA256) || record.PublisherID == "" {
		return nil, invalidInputPublication("recovery baseline checkpoint is invalid")
	}
	if _, _, err := s.readCatalogRecoveryBaseline(ctx, record.ManifestSHA256); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Runtime) initializeRecoveryBaseline(ctx context.Context) error {
	checkpoint, err := r.store.loadRecoveryBaseline(ctx)
	if err != nil || checkpoint == nil {
		return err
	}
	generation, _, err := r.store.readCatalogRecoveryBaseline(ctx, checkpoint.ManifestSHA256)
	if err != nil {
		return err
	}
	layers, err := decodeFleetRecoveryRecord(ctx, fleetRecoveryRecord{Baseline: generation}, layerSet{})
	if err != nil {
		return err
	}
	r.layers.embedded, r.layers.embeddedManifest, r.layers.fleetBaseline = layers.embedded, layers.embeddedManifest, layers.fleetBaseline
	r.recoveryBaseline = checkpoint.ManifestSHA256
	r.recoveryPublisher = checkpoint.PublisherID
	return nil
}

func materializationJournalDirectory(name string) bool {
	prefix := layerDirectoryName + "/" + materializationsDirectory
	return name == prefix || (path.Dir(name) == prefix && validFleetChecksum(path.Base(name)))
}

func materializationJournalFileLimit(name string) (int64, bool) {
	prefix := layerDirectoryName + "/" + materializationsDirectory
	if path.Dir(path.Dir(name)) != prefix || !validFleetChecksum(path.Base(path.Dir(name))) {
		return 0, false
	}
	switch path.Base(name) {
	case materializationPlanName:
		return MaxFleetRecoveryBytes, true
	case materializationReceiptName:
		return maxLayerBytes, true
	}
	return 0, false
}

func inspectMaterializationJournals(ctx context.Context, store *layerStore) error {
	parent, err := store.directory.ExistingChild(materializationsDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := parent.ReadDir()
	if err != nil {
		return err
	}
	if len(entries) > storage.DefaultRetentionScanEntries {
		return invalidInputPublication("materialization journal exceeds its entry limit")
	}
	for _, entry := range entries {
		if entry.Name() == privatefiles.PublicationDirectoryName && entry.IsDir() {
			continue
		}
		if !entry.IsDir() || !validFleetChecksum(entry.Name()) {
			return invalidInputPublication("materialization has an unknown journal")
		}
		journal, err := parent.ExistingChild(entry.Name())
		if err != nil {
			return err
		}
		if err := journal.CheckNoPendingPublications(ctx); err != nil {
			return err
		}
		plan, encoded, err := readMaterializationPlan(ctx, journal)
		if err != nil {
			return err
		}
		if plan == nil {
			continue
		}
		operation := plan.Receipt.OperationID
		if plan.Retention != nil {
			operation = plan.Retention.Receipt.OperationID
		}
		if fleetRecoveryChecksum([]byte(operation)) != entry.Name() {
			return invalidInputPublication("materialization journal identity differs from its directory")
		}
		if plan.Retention != nil {
			receipt, err := readCatalogRetentionReceipt(journal, *plan, encoded)
			if err != nil {
				return err
			}
			if receipt == nil {
				return invalidInputPublication("complete historical catalog retention before runtime startup")
			}
			continue
		}
		receipt, err := readMaterializationReceipt(journal, *plan, encoded)
		if err != nil {
			return err
		}
		if receipt == nil {
			return invalidInputPublication("complete catalog materialization before runtime startup")
		}
	}
	return ctx.Err()
}

func checkMaterializationIdentity(directory *privatefiles.Directory, records map[string]privatefiles.RecordReceipt) error {
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for _, name := range []string{ownerRecordName, instanceSeedFileName} {
		expected, found := records[name]
		if !found {
			return invalidInputPublication("materialization has no retained owner identity")
		}
		if err := privatefiles.CheckRecord(root, name, expected); err != nil {
			return err
		}
	}
	return nil
}

func checkMaterializedInputs(ctx context.Context, store *layerStore, request CatalogMaterializationRequest) error {
	record, err := readFleetRecovery(ctx, request.Inputs[request.Selected].Recovery.Inputs.Data)
	if err != nil {
		return err
	}
	baseline, err := catalogManifestChecksum(record.Baseline)
	if err != nil {
		return err
	}
	desired, err := materializationInputs(record, baseline)
	if err != nil {
		return err
	}
	actual, err := captureMaterializationFiles(ctx, store.directory)
	if err != nil {
		return err
	}
	for name, expected := range desired {
		if path.Dir(name) == inputPublicationDirectory {
			directory, base, err := materializationFileDirectory(store.directory, name, false)
			if err != nil {
				return err
			}
			got, err := directory.ReadFile(base, maxLayerBytes)
			if err != nil {
				return err
			}
			if !bytes.Equal(got, expected) {
				return invalidInputPublication("materialized checkpoint differs from selected inputs")
			}
			continue
		}
		if !bytes.Equal(actual[name], expected) {
			return invalidInputPublication("materialized catalog differs from selected inputs")
		}
		delete(actual, name)
	}
	if len(actual) != 0 {
		return invalidInputPublication("materialized catalog has additional active inputs")
	}
	return ctx.Err()
}
