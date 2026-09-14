package main

import (
	"context"
	"path/filepath"

	"github.com/agentstation/starmap/internal/catalog/publication"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

func restorePublication(ctx context.Context, options prepareOptions) (prepareReport, error) {
	if options.restoreReceipt == "" || options.restoreReceiptChecksum == "" || options.state == "" || options.stateChecksum == "" {
		return prepareReport{}, invalid("restore", "requires a receipt, checkpoint, and both trusted checksums")
	}
	if options.profile != "" || options.workspace != "" || options.baselineEmbedded {
		return prepareReport{}, invalid("restore", "cannot change source policy, workspace, or baseline")
	}
	if err := ctx.Err(); err != nil {
		return prepareReport{}, err
	}
	checkpoint, err := readBounded(options.state, publication.MaxPublicationStateBytes)
	if err != nil {
		return prepareReport{}, err
	}
	state, err := publication.RestoreState(ctx, checkpoint, options.stateChecksum)
	if err != nil {
		return prepareReport{}, err
	}
	if state.PublisherID() != options.publisher {
		return prepareReport{}, invalid("publisher_id", "does not match the restored checkpoint")
	}
	receipt, err := readBounded(options.restoreReceipt, artifact.MaxPublicationReceiptBytes)
	if err != nil {
		return prepareReport{}, err
	}
	if digest(receipt) != options.restoreReceiptChecksum {
		return prepareReport{}, invalid("restore_receipt", "does not match its trusted checksum")
	}
	decoded, err := artifact.DecodePublicationReceipt(receipt)
	if err != nil {
		return prepareReport{}, err
	}
	if decoded.RunID != options.runID {
		return prepareReport{}, invalid("run_id", "does not match the restored receipt")
	}
	output, err := filepath.Abs(options.output)
	if err != nil {
		return prepareReport{}, err
	}
	report, err := stagePrepared(ctx, output, preparedRecord{
		State:          publication.ReceiptRecord{Data: checkpoint, Checksum: options.stateChecksum},
		Receipt:        publication.ReceiptRecord{Data: receipt, Checksum: options.restoreReceiptChecksum},
		ReusedArtifact: true,
	})
	if err != nil {
		return prepareReport{}, err
	}
	report.Status = "restored"
	return report, nil
}
