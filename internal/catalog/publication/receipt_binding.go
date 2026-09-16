package publication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ReceiptRecord contains canonical receipt bytes and their immutable identity.
// Callers own both values and verify the checksum after storage or transport.
type ReceiptRecord struct {
	Data     []byte
	Checksum string
}

// BindReceipt binds admitted evidence to an artifact with the expected catalog semantics.
// The caller supplies the validated candidate checksum and authenticates retained inputs.
// This function checks artifact contents. Publisher provenance requires separate verification.
func BindReceipt(ctx context.Context, profile Profile, run Run, runID, expectedCatalogChecksum string, archive, statement []byte) (ReceiptRecord, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return ReceiptRecord{}, err
	}
	decision, err := Admit(profile, run)
	if err != nil {
		return ReceiptRecord{}, err
	}
	if !decision.Allowed {
		return ReceiptRecord{}, admissionError("receipt.admission", "requires admitted evidence for every required scope")
	}
	generation, err := artifact.Open(archive, statement)
	if err != nil {
		return ReceiptRecord{}, err
	}
	if err := ctx.Err(); err != nil {
		return ReceiptRecord{}, err
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		return ReceiptRecord{}, err
	}
	if semantic != expectedCatalogChecksum {
		return ReceiptRecord{}, admissionError("receipt.catalog_checksum", "artifact does not match the validated candidate")
	}
	receipt := artifact.PublicationReceipt{
		SchemaVersion: artifact.PublicationReceiptSchemaVersion, RunID: runID,
		StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, PolicyVersion: profile.Version,
		Artifact: artifact.PublicationArtifact{GenerationID: generation.Manifest.GenerationID,
			CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: receiptChecksum(archive)},
		FreshAcquisition: decision.FreshAcquisition,
	}
	for i, policy := range profile.Scopes {
		binding, err := receiptBinding(policy.Scope.Binding)
		if err != nil {
			return ReceiptRecord{}, err
		}
		result := decision.Scopes[i]
		receipt.Sources = append(receipt.Sources, artifact.PublicationSourceReceipt{
			Policy: artifact.PublicationScopePolicy{Source: policy.Scope.Source, Binding: binding,
				Required: policy.Required, Enabled: policy.Enabled, AllowMissing: policy.AllowMissing,
				AllowRecordQuarantine: policy.AllowRecordQuarantine, AllowStaleRetained: policy.AllowStaleRetained,
				MaxRetainedAge: policy.MaxRetainedAge, DisabledAction: string(policy.DisabledAction)},
			Attempt: string(result.Attempt), EvidenceKind: string(result.EvidenceKind), Observation: result.Evidence, Quarantine: result.Quarantine,
		})
	}
	data, err := artifact.EncodePublicationReceipt(receipt)
	if err != nil {
		return ReceiptRecord{}, err
	}
	if err := ctx.Err(); err != nil {
		return ReceiptRecord{}, err
	}
	return ReceiptRecord{Data: data, Checksum: receiptChecksum(data)}, nil
}

// The binding digest covers all declared selectors encoded as canonical JSON.
func receiptBinding(binding *sources.ProviderAcquisitionBinding) (*artifact.PublicationBinding, error) {
	if binding == nil {
		return nil, nil
	}
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(binding)
	if err != nil {
		return nil, pkgerrors.WrapResource("encode", "publication binding", "", err)
	}
	return &artifact.PublicationBinding{ID: binding.ID, Revision: binding.Revision, ProviderID: binding.ProviderID, Checksum: receiptChecksum(data)}, nil
}

func receiptChecksum(data []byte) string {
	digest := sha256.Sum256(data)
	return artifact.ChecksumPrefix + hex.EncodeToString(digest[:])
}
