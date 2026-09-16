package artifact

import "time"

const (
	// PublicationChannelSchemaVersion adds run receipts and verified source promotion.
	PublicationChannelSchemaVersion uint64 = 2
	// PublicationChannelName separates receipt-aware discovery from the legacy channel.
	PublicationChannelName = "catalog/v2"
	// PublicationChannelMediaType identifies the receipt-aware channel document.
	PublicationChannelMediaType = "application/vnd.agentstation.starmap.catalog-channel.v2+json"
	// PublicationReceiptTagPrefix names immutable run receipt releases.
	PublicationReceiptTagPrefix = "catalog-run-"
	// PublicationCheckpointFilename names the retained input asset on a run release.
	PublicationCheckpointFilename = "starmap-catalog-state.json"
	// PublicationCheckpointMediaType identifies a publisher replay checkpoint.
	PublicationCheckpointMediaType = "application/vnd.agentstation.starmap.catalog-checkpoint.v1+json"
	// MaxPublicationCheckpointBytes bounds retained publisher input bytes.
	MaxPublicationCheckpointBytes = 256 << 20
	gitCommitHexLength            = 40
)

// ChannelPublication binds a run receipt and the source revision that embeds its catalog.
type ChannelPublication struct {
	ReceiptTag   string       `json:"receipt_tag"`
	Receipt      ChannelAsset `json:"receipt"`
	SourceCommit string       `json:"source_commit"`
	Checkpoint   ChannelAsset `json:"checkpoint"`
}

// PublicationPromotion supplies verified inputs for one publication advance.
// The caller authenticates the receipt and verifies the merged embedded catalog.
type PublicationPromotion struct {
	Receipt                    []byte
	ReceiptChecksum            string
	Artifact                   PublicationArtifact
	SourceCommit               string
	ReceiptAttestationVerified bool
	EmbeddingVerified          bool
	Checkpoint                 ChannelAsset
	CheckpointVerified         bool
}

// PublicationReceiptTag returns the immutable release tag for exact receipt bytes.
func PublicationReceiptTag(receiptChecksum string) (string, error) {
	digest, err := digestHex(receiptChecksum)
	if err != nil {
		return "", err
	}
	return PublicationReceiptTagPrefix + digest, nil
}

// AdvancePublication binds a verified run to its artifact and merged source revision.
// Unchanged catalog facts reuse the previous artifact while selecting the new receipt.
func (c Channel) AdvancePublication(candidate Candidate, promotion PublicationPromotion, now time.Time) (Channel, AdvanceKind, error) {
	if !promotion.ReceiptAttestationVerified || !promotion.EmbeddingVerified || !promotion.CheckpointVerified {
		return Channel{}, "", channelValidation("publication.verification", nil, "requires verified receipt, checkpoint, and merged embedding")
	}
	if err := candidate.Validate(); err != nil {
		return Channel{}, "", err
	}
	if !candidate.Verification.Complete() {
		return Channel{}, "", channelValidation("candidate.verification", nil, "requires complete artifact verification")
	}
	receipt, err := VerifyPublicationReceipt(promotion.Receipt, promotion.ReceiptChecksum, promotion.Artifact)
	if err != nil {
		return Channel{}, "", err
	}
	if now.IsZero() || receipt.CompletedAt.After(now) {
		return Channel{}, "", channelValidation("publication.receipt", nil, "cannot confirm an unfinished future run")
	}
	if c.Sequence > 0 {
		if err := c.Validate(); err != nil {
			return Channel{}, "", err
		}
		if c.SchemaVersion != PublicationChannelSchemaVersion {
			return Channel{}, "", channelValidation("schema_version", c.SchemaVersion, "cannot reuse a legacy channel as publication state")
		}
		if c.Publication.Receipt.Checksum == promotion.ReceiptChecksum {
			if c.Publication.SourceCommit != promotion.SourceCommit || c.CatalogDigest != candidate.CatalogDigest || c.Publication.Receipt.SizeBytes != int64(len(promotion.Receipt)) || c.Publication.Checkpoint != promotion.Checkpoint {
				return Channel{}, "", channelValidation("publication.retry", nil, "must preserve the accepted run inputs")
			}
			if err := matchPublicationArtifact(c, promotion.Artifact); err != nil {
				return Channel{}, "", err
			}
			retry := c
			retry.Assets = copyChannelAssets(c.Assets)
			publication := *c.Publication
			retry.Publication = &publication
			return retry, AdvanceHeartbeat, nil
		}
	}
	legacy := c
	legacy.SchemaVersion = ChannelSchemaVersion
	legacy.Name = ChannelName
	legacy.Publication = nil
	next, kind, err := legacy.Advance(candidate, now)
	if err != nil {
		return Channel{}, "", err
	}
	if err := matchPublicationArtifact(next, promotion.Artifact); err != nil {
		return Channel{}, "", err
	}
	tag, err := PublicationReceiptTag(promotion.ReceiptChecksum)
	if err != nil {
		return Channel{}, "", err
	}
	next.SchemaVersion = PublicationChannelSchemaVersion
	next.Name = PublicationChannelName
	next.Publication = &ChannelPublication{
		ReceiptTag:   tag,
		Receipt:      ChannelAsset{Name: PublicationReceiptFilename, MediaType: PublicationReceiptMediaType, Checksum: promotion.ReceiptChecksum, SizeBytes: int64(len(promotion.Receipt))},
		SourceCommit: promotion.SourceCommit,
		Checkpoint:   promotion.Checkpoint,
	}
	if err := next.Validate(); err != nil {
		return Channel{}, "", err
	}
	return next, kind, nil
}

func matchPublicationArtifact(channel Channel, binding PublicationArtifact) error {
	if binding.GenerationID != channel.GenerationID || binding.CatalogChecksum != channel.CatalogDigest {
		return channelValidation("publication.artifact", nil, "must bind the selected catalog generation")
	}
	for _, asset := range channel.Assets {
		if asset.Name == Filename && asset.Checksum == binding.ArchiveChecksum {
			return nil
		}
	}
	return channelValidation("publication.artifact", nil, "must bind the selected archive checksum")
}

func (c Channel) validatePublication() error {
	if !publicationChecksum(c.CatalogDigest) || c.PublishedAt.After(c.ChannelUpdatedAt) {
		return channelValidation("publication.artifact", nil, "requires a prefixed catalog checksum and completed artifact publication")
	}
	if c.Publication == nil {
		return channelValidation("publication", nil, "is required for channel schema two")
	}
	p := c.Publication
	if p.Checkpoint.Name != PublicationCheckpointFilename || p.Checkpoint.MediaType != PublicationCheckpointMediaType || !publicationChecksum(p.Checkpoint.Checksum) || p.Checkpoint.SizeBytes <= 0 || p.Checkpoint.SizeBytes > MaxPublicationCheckpointBytes {
		return channelValidation("publication.checkpoint", nil, "must describe the verified retained input asset")
	}
	tag, err := PublicationReceiptTag(p.Receipt.Checksum)
	if err != nil || p.ReceiptTag != tag || !publicationChecksum(p.Receipt.Checksum) {
		return channelValidation("publication.receipt_tag", nil, "must bind the exact receipt checksum")
	}
	if p.Receipt.Name != PublicationReceiptFilename || p.Receipt.MediaType != PublicationReceiptMediaType || p.Receipt.SizeBytes <= 0 || p.Receipt.SizeBytes > maxPublicationReceiptBytes {
		return channelValidation("publication.receipt", nil, "must describe a bounded run receipt")
	}
	if len(p.SourceCommit) != gitCommitHexLength || !lowerHex(p.SourceCommit) {
		return channelValidation("publication.source_commit", nil, "must name the verified GitHub source commit")
	}
	for _, asset := range c.Assets {
		if asset.Name == Filename && asset.MediaType == MediaType && publicationChecksum(asset.Checksum) {
			return nil
		}
	}
	return channelValidation("publication.artifact", nil, "must name the selected catalog archive")
}

func lowerHex(value string) bool {
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
