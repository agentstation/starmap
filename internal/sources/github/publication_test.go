package github

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func TestGitHubPublicationVerifiesReceiptBeforeStateAdvance(t *testing.T) {
	server := newFixtureServer(t)
	published := publishCatalog(t, server, "generation-publication", 0)
	channel, receipt := publicationFixture(t, published)
	publishRun(t, server, channel, receipt)
	recorder := &recordingAttester{}
	source := newTestSource(t, server, WithChannel(artifact.PublicationChannelName), WithAttester(recorder.attest()))
	first, err := source.ReadChannel(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first.Publication == nil || first.Publication.RunID != receipt.RunID || first.SourceCommit != channel.Publication.SourceCommit || first.ChannelUpdatedAt != channel.ChannelUpdatedAt {
		t.Fatal("missing verified publication metadata")
	}
	if first.Budget.Requests != requestsPerChangedCycle+3 {
		t.Fatalf("requests=%d", first.Budget.Requests)
	}
	originalPayload := append([]byte(nil), first.Generation.Payload...)
	originalObservation := first.Publication.Sources[0].Observation.ObservedAt
	receipt.RunID = "run-retained"
	receipt.StartedAt = receipt.CompletedAt.Add(time.Minute)
	receipt.CompletedAt = receipt.StartedAt
	receipt.Sources[0].Attempt = "failed"
	receipt.Sources[0].EvidenceKind = "retained"
	receipt.FreshAcquisition = false
	channel = advanceRun(t, channel, published, receipt)
	publishRun(t, server, channel, receipt)
	second, err := source.ReadChannel(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if second.Publication.FreshAcquisition || second.Publication.Sources[0].Observation.ObservedAt != originalObservation || second.Publication.RunID == first.Publication.RunID {
		t.Fatal("retained receipt invented source freshness")
	}
	if second.GenerationID != first.GenerationID || !bytes.Equal(second.Generation.Payload, originalPayload) || second.PublishedAt != first.PublishedAt || !second.ChannelUpdatedAt.After(first.ChannelUpdatedAt) {
		t.Fatal("confirmation changed the historical catalog")
	}
}

func TestGitHubPublicationRefusalPreservesAcceptedState(t *testing.T) {
	for _, name := range []string{"missing receipt", "wrong size", "wrong bytes", "wrong artifact", "receipt provenance", "duplicate asset", "same sequence", "schema downgrade"} {
		t.Run(name, func(t *testing.T) {
			server := newFixtureServer(t)
			published := publishCatalog(t, server, "generation-publication", 0)
			channel, receipt := publicationFixture(t, published)
			publishRun(t, server, channel, receipt)
			recorder := &recordingAttester{reject: map[string]error{}}
			source := newTestSource(t, server, WithChannel(artifact.PublicationChannelName), WithAttester(recorder.attest()))
			if _, err := source.ReadChannel(t.Context()); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(source.state.path)
			if err != nil {
				t.Fatal(err)
			}
			receipt.RunID = "run-next"
			channel = advanceRun(t, channel, published, receipt)
			raw, err := artifact.EncodePublicationReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			assets := []assetFixture{{Name: artifact.PublicationReceiptFilename, Body: raw}}
			switch name {
			case "missing receipt":
				assets = nil
			case "wrong size":
				assets[0].Body = append(assets[0].Body, ' ')
			case "wrong bytes":
				assets[0].Body = bytes.Replace(assets[0].Body, []byte("run-next"), []byte("bad-next"), 1)
			case "wrong artifact":
				receipt.Artifact.PayloadChecksum = "sha256:" + strings.Repeat("0", 64)
				raw, err = artifact.EncodePublicationReceipt(receipt)
				if err != nil {
					t.Fatal(err)
				}
				assets[0].Body = raw
				channel.Publication.Receipt.Checksum = "sha256:" + hexDigest(raw)
				channel.Publication.ReceiptTag, err = artifact.PublicationReceiptTag(channel.Publication.Receipt.Checksum)
				if err != nil {
					t.Fatal(err)
				}
				channel.Publication.Receipt.SizeBytes = int64(len(raw))
			case "receipt provenance":
				recorder.reject[hexDigest(raw)] = errors.New("fixture refused receipt")
			case "duplicate asset":
				assets = append(assets, assets[0])
			case "same sequence":
				channel.Sequence--
				channel.Publication.SourceCommit = strings.Repeat("b", 40)
			case "schema downgrade":
				channel.SchemaVersion = artifact.ChannelSchemaVersion
				channel.Name = artifact.ChannelName
				channel.Publication = nil
			}
			if channel.Publication != nil {
				server.publish(releaseFixture{Tag: channel.Publication.ReceiptTag, Assets: assets})
			}
			encoded, err := artifact.EncodeChannel(channel)
			if err != nil {
				t.Fatal(err)
			}
			server.publishChannel(encoded)
			if _, err := source.ReadChannel(t.Context()); err == nil {
				t.Fatal("invalid publication advanced state")
			}
			after, err := os.ReadFile(source.state.path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refusal changed accepted state: %v", err)
			}
		})
	}
}

func publicationFixture(t *testing.T, published publication) (artifact.Channel, artifact.PublicationReceipt) {
	t.Helper()
	bundle, err := artifact.Build(published.Generation)
	if err != nil {
		t.Fatal(err)
	}
	observation := published.Generation.Manifest.SourceObservations[0]
	observation.Source = evidence.ModelsDevHTTPID
	observation.Completeness = evidence.ObservationCompletenessComplete
	observation.Status = evidence.ObservationStatusSucceeded
	receipt := artifact.PublicationReceipt{SchemaVersion: artifact.PublicationReceiptSchemaVersion, RunID: "run-first", StartedAt: observation.ObservedAt, CompletedAt: observation.ObservedAt, PolicyVersion: "public-fixture", FreshAcquisition: true, Artifact: artifact.PublicationArtifact{GenerationID: published.Generation.Manifest.GenerationID, CatalogChecksum: semanticChecksum(t, published.Generation), PayloadChecksum: published.Generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}, Sources: []artifact.PublicationSourceReceipt{{Policy: artifact.PublicationScopePolicy{Source: observation.Source, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: "preserve"}, Attempt: "succeeded", EvidenceKind: "fresh", Observation: &observation}}}
	return advanceRun(t, artifact.Channel{}, published, receipt), receipt
}

func advanceRun(t *testing.T, previous artifact.Channel, published publication, receipt artifact.PublicationReceipt) artifact.Channel {
	t.Helper()
	raw, err := artifact.EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	candidate := artifact.Candidate{GenerationID: published.Generation.Manifest.GenerationID, Tag: published.Tag, CatalogDigest: semanticChecksum(t, published.Generation), PublishedAt: receipt.CompletedAt, Assets: channelAssetsOf(published.Assets), Verification: artifact.ReleaseVerification{AssetsPresent: true, ChecksumsMatch: true, AttestationVerified: true}}
	now := receipt.CompletedAt
	if !now.After(previous.ChannelUpdatedAt) {
		now = previous.ChannelUpdatedAt.Add(time.Second)
	}
	channel, _, err := previous.AdvancePublication(candidate, artifact.PublicationPromotion{Receipt: raw, ReceiptChecksum: "sha256:" + hexDigest(raw), Artifact: receipt.Artifact, SourceCommit: strings.Repeat("a", 40), ReceiptAttestationVerified: true, EmbeddingVerified: true, CheckpointVerified: true, Checkpoint: artifact.ChannelAsset{Name: artifact.PublicationCheckpointFilename, MediaType: artifact.PublicationCheckpointMediaType, Checksum: "sha256:" + strings.Repeat("c", 64), SizeBytes: 100}}, now)
	if err != nil {
		t.Fatal(err)
	}
	return channel
}

func publishRun(t *testing.T, server *fixtureServer, channel artifact.Channel, receipt artifact.PublicationReceipt) {
	t.Helper()
	raw, err := artifact.EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	server.publish(releaseFixture{Tag: channel.Publication.ReceiptTag, Assets: []assetFixture{{Name: artifact.PublicationReceiptFilename, Body: raw}}})
	encoded, err := artifact.EncodeChannel(channel)
	if err != nil {
		t.Fatal(err)
	}
	server.setChannelRef(artifact.PublicationChannelName)
	server.publishChannel(encoded)
}
