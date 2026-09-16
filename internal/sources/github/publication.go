package github

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

func (s *Source) readPublicationReceipt(ctx context.Context, refresh *cycle, channel artifact.Channel, release Release) (artifact.PublicationReceipt, error) {
	publication := channel.Publication
	_, document, err := refresh.releaseByTag(ctx, publication.ReceiptTag, "")
	if err != nil {
		return artifact.PublicationReceipt{}, err
	}
	asset, err := document.asset(publication.Receipt.Name)
	if err != nil {
		return artifact.PublicationReceipt{}, err
	}
	count := 0
	for _, item := range document.Assets {
		if item.Name == publication.Receipt.Name {
			count++
		}
	}
	if count != 1 || asset.Size != publication.Receipt.SizeBytes {
		return artifact.PublicationReceipt{}, sourceValidation("publication.receipt", nil, "must uniquely match the attested receipt size")
	}
	answer, err := refresh.getBounded(ctx, asset.URL, acceptBinary, "", resourceAsset, asset.Size)
	if err != nil {
		return artifact.PublicationReceipt{}, err
	}
	if err := refresh.checkStatus(answer, resourceAsset, publication.Receipt.Name); err != nil {
		return artifact.PublicationReceipt{}, err
	}
	if int64(len(answer.Body)) != asset.Size || digestHex(answer.Body) != trimChecksum(publication.Receipt.Checksum) {
		return artifact.PublicationReceipt{}, sourceValidation("publication.receipt", nil, "does not match the attested receipt bytes")
	}
	if _, err := refresh.verifyBytes(ctx, answer.Body); err != nil {
		return artifact.PublicationReceipt{}, err
	}
	receipt, err := artifact.VerifyPublicationReceipt(answer.Body, publication.Receipt.Checksum, artifact.PublicationArtifact{
		GenerationID:    release.GenerationID,
		CatalogChecksum: release.CatalogDigest,
		PayloadChecksum: release.Generation.Manifest.Payload.Checksum,
		ArchiveChecksum: release.ArchiveChecksum,
	})
	if err != nil {
		return artifact.PublicationReceipt{}, err
	}
	if receipt.CompletedAt.After(channel.ChannelUpdatedAt) {
		return artifact.PublicationReceipt{}, sourceValidation("publication.completed_at", nil, "cannot follow channel confirmation")
	}
	return receipt, nil
}
