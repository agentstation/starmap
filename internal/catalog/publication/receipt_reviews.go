package publication

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

// bindReviewReceipt preserves current review evidence when catalog artifact bytes stay unchanged.
func bindReviewReceipt(record ReceiptRecord, manifest catalogs.GenerationManifest) (ReceiptRecord, error) {
	receipt, err := artifact.DecodePublicationReceipt(record.Data)
	if err != nil {
		return ReceiptRecord{}, err
	}
	receipt.Reviews = slices.Clone(manifest.ReviewCandidates)
	referenced := make(map[string]bool, len(receipt.Reviews))
	for _, review := range receipt.Reviews {
		referenced[review.SourceObservationID] = true
	}
	for _, observation := range manifest.SourceObservations {
		if referenced[observation.ObservationID] {
			receipt.ReviewObservations = append(receipt.ReviewObservations, observation)
		}
	}
	data, err := artifact.EncodePublicationReceipt(receipt)
	if err != nil {
		return ReceiptRecord{}, err
	}
	return ReceiptRecord{Data: data, Checksum: receiptChecksum(data)}, nil
}
