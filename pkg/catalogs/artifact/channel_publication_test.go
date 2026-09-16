package artifact

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPublicationChannelRetainsArtifactAndRetriesExactRun(t *testing.T) {
	candidate, promotion, receipt := publicationChannelFixture(t)
	first, kind, err := (Channel{}).AdvancePublication(candidate, promotion, receipt.CompletedAt)
	if err != nil || kind != AdvancePromotion {
		t.Fatalf("first publication: %s, %v", kind, err)
	}
	firstBytes, err := EncodeChannel(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, now := range []time.Time{receipt.CompletedAt, receipt.CompletedAt.Add(time.Minute)} {
		retry, _, err := first.AdvancePublication(candidate, promotion, now)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		retryBytes, err := EncodeChannel(retry)
		if err != nil || !bytes.Equal(firstBytes, retryBytes) {
			t.Fatalf("retry changed accepted channel: %v", err)
		}
		retry.Assets[0].Name = "changed"
		retry.Publication.SourceCommit = "changed"
		unchanged, _ := EncodeChannel(first)
		if !bytes.Equal(firstBytes, unchanged) {
			t.Fatal("retry aliases accepted channel")
		}
	}
	receipt.RunID = "next-run"
	receipt.StartedAt = receipt.CompletedAt.Add(time.Minute)
	receipt.CompletedAt = receipt.StartedAt
	receipt.FreshAcquisition = false
	receipt.Sources[0].Attempt = "failed"
	receipt.Sources[0].EvidenceKind = "retained"
	promotion.Receipt, err = EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	promotion.ReceiptChecksum = checksum(promotion.Receipt)
	next, kind, err := first.AdvancePublication(candidate, promotion, receipt.CompletedAt)
	if err != nil || kind != AdvanceHeartbeat {
		t.Fatalf("retained publication: %s, %v", kind, err)
	}
	if next.Sequence != first.Sequence+1 || next.GenerationID != first.GenerationID || next.PublishedAt != first.PublishedAt || next.Publication.ReceiptTag == first.Publication.ReceiptTag {
		t.Fatal("new receipt did not retain the selected artifact")
	}
	if _, _, err := next.Advance(candidate, receipt.CompletedAt.Add(time.Minute)); err == nil {
		t.Fatal("legacy advance discarded publication evidence")
	}
}

func TestPublicationChannelRejectsUnverifiedInputs(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Candidate, *PublicationPromotion)
	}{
		{"receipt provenance", func(_ *Candidate, p *PublicationPromotion) { p.ReceiptAttestationVerified = false }},
		{"merged input", func(_ *Candidate, p *PublicationPromotion) { p.EmbeddingVerified = false }},
		{"checkpoint verification", func(_ *Candidate, p *PublicationPromotion) { p.CheckpointVerified = false }},
		{"checkpoint digest", func(_ *Candidate, p *PublicationPromotion) { p.Checkpoint.Checksum = "invalid" }},
		{"checkpoint size", func(_ *Candidate, p *PublicationPromotion) {
			p.Checkpoint.SizeBytes = MaxPublicationCheckpointBytes + 1
		}},
		{"artifact provenance", func(c *Candidate, _ *PublicationPromotion) { c.Verification.AttestationVerified = false }},
		{"receipt bytes", func(_ *Candidate, p *PublicationPromotion) { p.Receipt = append(p.Receipt, ' ') }},
		{"receipt digest", func(_ *Candidate, p *PublicationPromotion) { p.ReceiptChecksum = "sha256:" + strings.Repeat("0", 64) }},
		{"generation", func(_ *Candidate, p *PublicationPromotion) { p.Artifact.GenerationID = "generation-other" }},
		{"catalog", func(_ *Candidate, p *PublicationPromotion) {
			p.Artifact.CatalogChecksum = "sha256:" + strings.Repeat("0", 64)
		}},
		{"archive", func(_ *Candidate, p *PublicationPromotion) {
			p.Artifact.ArchiveChecksum = "sha256:" + strings.Repeat("0", 64)
		}},
		{"payload", func(_ *Candidate, p *PublicationPromotion) {
			p.Artifact.PayloadChecksum = "sha256:" + strings.Repeat("0", 64)
		}},
		{"commit", func(_ *Candidate, p *PublicationPromotion) { p.SourceCommit = "main" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, promotion, receipt := publicationChannelFixture(t)
			tt.change(&candidate, &promotion)
			if _, _, err := (Channel{}).AdvancePublication(candidate, promotion, receipt.CompletedAt); err == nil {
				t.Fatal("invalid input advanced channel")
			}
		})
	}
	candidate, promotion, receipt := publicationChannelFixture(t)
	if _, _, err := (Channel{}).AdvancePublication(candidate, promotion, receipt.CompletedAt.Add(-time.Nanosecond)); err == nil {
		t.Fatal("future receipt advanced channel")
	}
	legacy, _, err := (Channel{}).Advance(candidate, receipt.CompletedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := legacy.AdvancePublication(candidate, promotion, receipt.CompletedAt.Add(time.Minute)); err == nil {
		t.Fatal("legacy state silently migrated")
	}
}

func TestPublicationChannelStrictEncoding(t *testing.T) {
	candidate, promotion, receipt := publicationChannelFixture(t)
	channel, _, err := (Channel{}).AdvancePublication(candidate, promotion, receipt.CompletedAt)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeChannel(channel)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeChannel(encoded)
	if err != nil || decoded.Publication.SourceCommit != promotion.SourceCommit {
		t.Fatalf("round trip: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	fields["publication"] = json.RawMessage("null")
	missing, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{append([]byte(" "), encoded...), bytes.Replace(encoded, []byte(`"sequence": 1,`), []byte(`"sequence": 1, "sequence": 1,`), 1), missing} {
		if _, err := DecodeChannel(data); err == nil {
			t.Fatal("noncanonical publication channel passed")
		}
	}
	legacy, _, err := (Channel{}).Advance(candidate, receipt.CompletedAt)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeChannel(legacy)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte("{\n"), []byte("{\n  \"publication\": null,\n"), 1)
	if _, err := DecodeChannel(raw); err == nil {
		t.Fatal("legacy channel accepted new field")
	}
	if IsReleaseTag(channel.Publication.ReceiptTag) {
		t.Fatal("receipt tag entered catalog namespace")
	}
}

func publicationChannelFixture(t *testing.T) (Candidate, PublicationPromotion, PublicationReceipt) {
	t.Helper()
	receipt := publicationReceiptFixture(t)
	bundle, err := Build(artifactFixtureGeneration(t))
	if err != nil {
		t.Fatal(err)
	}
	tag, err := ReleaseTag(receipt.Artifact.CatalogChecksum)
	if err != nil {
		t.Fatal(err)
	}
	candidate := Candidate{GenerationID: receipt.Artifact.GenerationID, Tag: tag, CatalogDigest: receipt.Artifact.CatalogChecksum, PublishedAt: receipt.CompletedAt, Assets: []ChannelAsset{{Name: Filename, MediaType: MediaType, Checksum: bundle.Checksum, SizeBytes: int64(len(bundle.Data))}}, Verification: ReleaseVerification{AssetsPresent: true, ChecksumsMatch: true, AttestationVerified: true}}
	raw, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	promotion := PublicationPromotion{Receipt: raw, ReceiptChecksum: checksum(raw), Artifact: receipt.Artifact, SourceCommit: strings.Repeat("a", 40), ReceiptAttestationVerified: true, EmbeddingVerified: true, CheckpointVerified: true, Checkpoint: ChannelAsset{Name: PublicationCheckpointFilename, MediaType: PublicationCheckpointMediaType, Checksum: "sha256:" + strings.Repeat("c", 64), SizeBytes: 100}}
	return candidate, promotion, receipt
}
