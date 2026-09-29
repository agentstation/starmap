package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func retainedMaterializationBinding(ctx context.Context, request CatalogRetainedMaterializationRequest, opts []Option) (string, error) {
	if ctx == nil {
		return "", invalidInputPublication("retained selection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !filepath.IsAbs(request.Directory) || filepath.Clean(request.Directory) != request.Directory || !validRecoveryOperationID(request.OperationID) || !validRecoveryOperationID(request.TransferID) || request.Selected < 0 || request.Selected >= storage.DefaultRetentionScanEntries || len(request.Batches) == 0 || len(request.Batches) > storage.DefaultRetentionScanEntries {
		return "", invalidInputPublication("retained selection requires complete bounded batch references")
	}
	if err := request.Owner.Validate(); err != nil {
		return "", err
	}
	seen := map[string]bool{}
	for _, batch := range request.Batches {
		if !validRecoveryOperationID(batch.OperationID) || !validFleetChecksum(batch.ReceiptSHA256) || seen[batch.OperationID] {
			return "", invalidInputPublication("retained selection contains an invalid or repeated batch")
		}
		seen[batch.OperationID] = true
	}
	config := defaults()
	if _, err := config.apply(opts...); err != nil {
		return "", err
	}
	config.resolve()
	if err := config.validate(); err != nil {
		return "", err
	}
	description, err := describeSources(config)
	if err != nil {
		return "", err
	}
	// Bind source and acquisition settings without consulting the current binary baseline.
	customIdentity := ""
	if config.customSource != nil {
		customIdentity = config.customSource.Identity()
	}
	probe := Runtime{config: *config}
	compatibility, err := fleetLayerCompatibility(layerSet{requireAuthority: probe.requiresAuthority(), providerBindings: config.providerBindings, acquisitionSources: config.acquisitionSources, publisherAliases: config.source.Aliases, sourceConfiguration: description})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal([]any{request, config.source, compatibility, customIdentity}, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
	if err != nil {
		return "", err
	}
	return fleetRecoveryChecksum(data), nil
}

func completedRetainedMaterialization(ctx context.Context, request CatalogRetainedMaterializationRequest, binding string) (receipt CatalogMaterializationReceipt, complete bool, resultErr error) {
	owner := CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: request.OperationID}
	directory, lock, entry, store, err := openCatalogRecoveryOwner(ctx, owner)
	if err != nil {
		return receipt, false, err
	}
	defer func() {
		resultErr = stderrors.Join(resultErr, lock.Close())
		if resultErr != nil {
			receipt = CatalogMaterializationReceipt{}
			complete = false
		}
	}()
	return completedRetainedMaterializationAt(ctx, request, binding, directory, entry, store)
}

func completedRetainedMaterializationAt(ctx context.Context, request CatalogRetainedMaterializationRequest, binding string, directory *privatefiles.Directory, entry privatefiles.EntryReceipt, store *layerStore) (receipt CatalogMaterializationReceipt, complete bool, resultErr error) {
	journals, err := store.directory.ExistingChild(materializationsDirectory)
	if os.IsNotExist(err) {
		return receipt, false, nil
	}
	if err != nil {
		return receipt, false, err
	}
	journal, err := journals.ExistingChild(fleetRecoveryChecksum([]byte(request.OperationID)))
	if os.IsNotExist(err) {
		return receipt, false, nil
	}
	if err != nil {
		return receipt, false, err
	}
	if err := journal.CheckNoPendingPublications(ctx); err != nil {
		return receipt, false, err
	}
	plan, encoded, err := readMaterializationPlan(ctx, journal)
	if err != nil || plan == nil {
		return receipt, false, err
	}
	if plan.Retention != nil || plan.RetainedBindingSHA256 != binding || plan.Receipt.DirectoryIdentity != entry {
		return receipt, false, invalidInputPublication("retained selection operation, target, or settings changed")
	}
	found, err := readMaterializationReceipt(journal, *plan, encoded)
	if err != nil || found == nil {
		return receipt, false, err
	}
	if err := checkMaterializationIdentity(directory, plan.Continuity); err != nil {
		return receipt, false, err
	}
	return *found, true, ctx.Err()

}

func readRetainedCatalogSelection(ctx context.Context, request CatalogRetainedMaterializationRequest) (input CatalogMaterializationInput, resultErr error) {
	owner := CatalogMaterializationRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, OperationID: request.OperationID}
	directory, lock, entry, store, err := openCatalogRecoveryOwner(ctx, owner)
	if err != nil {
		return CatalogMaterializationInput{}, err
	}
	defer func() {
		resultErr = stderrors.Join(resultErr, lock.Close())
		if resultErr != nil {
			input = CatalogMaterializationInput{}
		}
	}()
	return readRetainedCatalogSelectionAt(ctx, request, directory, entry, store)
}

func readRetainedCatalogSelectionAt(ctx context.Context, request CatalogRetainedMaterializationRequest, directory *privatefiles.Directory, entry privatefiles.EntryReceipt, store *layerStore) (CatalogMaterializationInput, error) {
	transfer, err := catalogTransferDirectory(store, request.TransferID, false)
	if err != nil {
		return CatalogMaterializationInput{}, err
	}
	data, err := transfer.ReadFile("manifest.json", maxLayerBytes)
	if err != nil {
		return CatalogMaterializationInput{}, err
	}
	var manifest retainedCatalogManifest
	if err := json.Unmarshal(data, &manifest, json.RejectUnknownMembers(true)); err != nil {
		return CatalogMaterializationInput{}, err
	}
	expected, err := catalogRetentionManifestBytes(request.TransferID, manifest.Entries)
	if err != nil || manifest.Version != materializationVersion || manifest.TransferID != request.TransferID || !bytes.Equal(expected, data) || request.Selected >= len(manifest.Entries) {
		return CatalogMaterializationInput{}, invalidInputPublication("retained selection differs from its full transfer manifest")
	}
	var selected CatalogRetainedRecord
	covered := 0
	for _, batch := range request.Batches {
		receipt, err := readRetainedBatch(ctx, directory, store, batch)
		if err != nil {
			return CatalogMaterializationInput{}, err
		}
		if receipt.TransferID != request.TransferID || receipt.ManifestSHA256 != fleetRecoveryChecksum(data) || receipt.DirectoryIdentity != entry || receipt.BatchStart != covered || len(receipt.Records) > len(manifest.Entries)-covered {
			return CatalogMaterializationInput{}, invalidInputPublication("retained batches have changed ownership, gaps, or overlaps")
		}
		for _, record := range receipt.Records {
			hash := sha256.New()
			if _, err := transfer.CopyFile(ctx, record.RecordSHA256+".json.gz", hash, maxRetainedCatalogRecordBytes); err != nil {
				return CatalogMaterializationInput{}, err
			}
			if hex.EncodeToString(hash.Sum(nil)) != record.RecordSHA256 {
				return CatalogMaterializationInput{}, invalidInputPublication("retained inventory envelope changed")
			}
			if record.Entry != manifest.Entries[covered] {
				return CatalogMaterializationInput{}, invalidInputPublication("retained batch differs from its complete manifest")
			}
			if covered == request.Selected {
				selected = record
			}
			covered++
		}
	}
	if covered != len(manifest.Entries) {
		return CatalogMaterializationInput{}, invalidInputPublication("retained batches do not cover the complete manifest")
	}
	return readRetainedCatalogInput(ctx, transfer, selected)

}

func readRetainedBatch(ctx context.Context, directory *privatefiles.Directory, store *layerStore, batch CatalogRetentionBatch) (*CatalogRetentionReceipt, error) {
	journals, err := store.directory.ExistingChild(materializationsDirectory)
	if err != nil {
		return nil, err
	}
	journal, err := journals.ExistingChild(fleetRecoveryChecksum([]byte(batch.OperationID)))
	if err != nil {
		return nil, err
	}
	if err := journal.CheckNoPendingPublications(ctx); err != nil {
		return nil, err
	}
	plan, encoded, err := readMaterializationPlan(ctx, journal)
	if err != nil {
		return nil, err
	}
	if plan == nil || plan.Retention == nil || plan.Retention.Receipt.OperationID != batch.OperationID {
		return nil, invalidInputPublication("retained batch has no matching retention journal")
	}
	receipt, err := readCatalogRetentionReceipt(journal, *plan, encoded)
	if err != nil || receipt == nil {
		return nil, stderrors.Join(err, invalidInputPublication("retained batch is incomplete"))
	}
	raw, err := journal.ReadFile(materializationReceiptName, maxLayerBytes)
	if err != nil {
		return nil, err
	}
	if fleetRecoveryChecksum(raw) != batch.ReceiptSHA256 {
		return nil, invalidInputPublication("retained batch completion differs from its reference")
	}
	if err := checkMaterializationIdentity(directory, plan.Continuity); err != nil {
		return nil, err
	}
	return receipt, ctx.Err()
}
