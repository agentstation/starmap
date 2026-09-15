package runtime

import (
	"encoding/hex"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
)

const publicationSourceCommitBytes = 20

// UpstreamPublication returns a caller-owned copy of the accepted source run record.
// It returns nil when the source supplies no run receipt. The artifact manifest stays unchanged.
func (r *Runtime) UpstreamPublication() *SourcePublication {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	var publication *SourcePublication
	if r.layers.source != nil {
		publication = r.layers.source.Publication
	}
	r.mu.RUnlock()
	if publication == nil {
		return nil
	}
	owned := publication.Copy()
	return &owned
}

func (s *sourceLayer) publicationStatus() *PublicationStatus {
	if s.Publication == nil {
		return nil
	}
	p := s.Publication
	return &PublicationStatus{RunID: p.Receipt.RunID, ReceiptChecksum: p.Checksum, SourceCommit: p.SourceCommit,
		GenerationID: p.Receipt.Artifact.GenerationID, CompletedAt: p.Receipt.CompletedAt,
		FreshAcquisition: p.Receipt.FreshAcquisition, SourceCount: len(p.Receipt.Sources), ReviewCount: len(p.Receipt.Reviews)}
}

func (s *sourceLayer) validatePublication(generation catalogs.Generation) error {
	if s.Publication == nil {
		return nil
	}
	p := s.Publication
	commit, err := hex.DecodeString(p.SourceCommit)
	if err != nil || len(commit) != publicationSourceCommitBytes || p.SourceCommit != strings.ToLower(p.SourceCommit) {
		return &errors.ValidationError{Field: "source_layer.publication.source_commit", Message: "must name the attested source commit"}
	}
	data, err := artifact.EncodePublicationReceipt(p.Receipt)
	if err != nil {
		return err
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		return err
	}
	// The source verifies archive bytes and publisher provenance before retention.
	// Replay checks the retained generation and exact receipt checksum again.
	_, err = artifact.VerifyPublicationReceipt(data, p.Checksum, artifact.PublicationArtifact{
		GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic,
		PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: p.Receipt.Artifact.ArchiveChecksum,
	})
	if err != nil {
		return err
	}
	if s.ChannelUpdatedAt.IsZero() || p.Receipt.CompletedAt.After(s.ChannelUpdatedAt) {
		return &errors.ValidationError{Field: "source_layer.publication.completed_at", Message: "cannot follow channel confirmation"}
	}
	return nil
}
