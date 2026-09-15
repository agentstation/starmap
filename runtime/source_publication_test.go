package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRuntimePublicationRetainsCurrentReceiptAndHistoricalArtifact(t *testing.T) {
	read := sourcePublicationFixture(t)
	source := newStubSource("verified-publication")
	source.replies = []SourceRead{read}
	directory := privateRuntimeDirectory(t)
	store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	options := []Option{WithSource(source), WithStateDirectory(directory), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	initial := connected.State()
	originalManifest := read.Generation.Manifest.Copy()
	originalPayload := bytes.Clone(read.Generation.Payload)
	first := connected.UpstreamPublication()
	if first == nil || !reflect.DeepEqual(*first, *read.Publication) {
		t.Fatal("runtime lost verified publication")
	}
	first.Receipt.Sources[0].Observation.ObservedAt = time.Time{}
	first.Receipt.Reviews[0].Reason = "changed"
	first.Receipt.ReviewObservations[0].ObservationID = "changed"
	first.SourceCommit = "changed"
	if !reflect.DeepEqual(*connected.UpstreamPublication(), *read.Publication) {
		t.Fatal("caller changed retained publication")
	}
	status := connected.Status().UpstreamPublication
	if status == nil || status.ReviewCount != 1 || status.RunID != read.Publication.Receipt.RunID || status.ReceiptChecksum != read.Publication.Checksum {
		t.Fatal("status lost run summary")
	}
	status.RunID = "changed"
	if connected.Status().UpstreamPublication.RunID == "changed" {
		t.Fatal("caller changed publication summary")
	}

	next := read
	publication := read.Publication.Copy()
	next.Publication = &publication
	next.Publication.Receipt.RunID = "retained-next"
	next.Publication.Receipt.StartedAt = read.Publication.Receipt.CompletedAt.Add(time.Minute)
	next.Publication.Receipt.CompletedAt = next.Publication.Receipt.StartedAt
	next.Publication.Receipt.FreshAcquisition = false
	next.Publication.Receipt.Sources[0].Attempt = "failed"
	next.Publication.Receipt.Sources[0].EvidenceKind = "retained"
	next.Publication.Receipt.Reviews = nil
	next.Publication.Receipt.ReviewObservations = nil
	next.ChannelUpdatedAt = next.Publication.Receipt.CompletedAt
	setPublicationChecksum(t, next.Publication)
	source.mu.Lock()
	source.replies = []SourceRead{next}
	source.mu.Unlock()
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if connected.State().GenerationID != initial.GenerationID || connected.State().PayloadChecksum != initial.PayloadChecksum {
		t.Fatal("receipt refresh changed effective catalog identity")
	}
	current := connected.UpstreamPublication()
	if current.Receipt.FreshAcquisition || len(current.Receipt.Reviews) != 0 || current.Receipt.Sources[0].Observation.ObservedAt != read.Publication.Receipt.Sources[0].Observation.ObservedAt {
		t.Fatal("receipt refresh invented source freshness or retained obsolete reviews")
	}
	raw, err := os.ReadFile(filepath.Join(directory, layerDirectoryName, sourceLayerFileName))
	if err != nil {
		t.Fatal(err)
	}
	var saved sourceLayer
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*saved.Manifest, originalManifest) || !bytes.Equal(saved.Payload, originalPayload) {
		t.Fatal("receipt refresh changed retained artifact bytes")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	source.replies = []SourceRead{{Health: HealthUnavailable}}
	source.mu.Unlock()
	restarted := openTestRuntime(t, options...)
	if !reflect.DeepEqual(restarted.UpstreamPublication(), current) {
		t.Fatal("restart lost accepted run evidence")
	}
	if restarted.Status().UpstreamPublication.ReviewCount != 0 || restarted.Status().ChannelUpdatedAt != next.ChannelUpdatedAt {
		t.Fatal("restart changed run status or channel time")
	}
}

func TestRuntimePublicationRefusesMismatchedRetainedEvidence(t *testing.T) {
	read := sourcePublicationFixture(t)
	source := newStubSource("verified-publication")
	source.replies = []SourceRead{read}
	directory := privateRuntimeDirectory(t)
	connected := openTestRuntime(t, WithSource(source), WithStateDirectory(directory))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	original := connected.State()
	path := filepath.Join(directory, layerDirectoryName, sourceLayerFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"checksum", "generation", "payload", "source commit", "confirmation time", "missing manifest"} {
		t.Run(name, func(t *testing.T) {
			next := read
			next.Generation = read.Generation.Copy()
			publication := read.Publication.Copy()
			next.Publication = &publication
			switch name {
			case "checksum":
				publication.Checksum = "sha256:" + strings.Repeat("0", 64)
			case "generation":
				publication.Receipt.Artifact.GenerationID = "generation-other"
				setPublicationChecksum(t, &publication)
			case "payload":
				publication.Receipt.Artifact.PayloadChecksum = "sha256:" + strings.Repeat("0", 64)
				setPublicationChecksum(t, &publication)
			case "source commit":
				publication.SourceCommit = "main"
			case "confirmation time":
				next.ChannelUpdatedAt = publication.Receipt.CompletedAt.Add(-time.Second)
			case "missing manifest":
				next.Generation.Manifest.ManifestVersion = 0
			}
			source.mu.Lock()
			source.replies = []SourceRead{next}
			source.mu.Unlock()
			if _, err := connected.RefreshSource(t.Context()); err == nil {
				t.Fatal("invalid run evidence became active")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) || connected.State().GenerationID != original.GenerationID {
				t.Fatal("refused receipt changed retained state")
			}
		})
	}
}

func sourcePublicationFixture(t *testing.T) SourceRead {
	t.Helper()
	generation, _, _, at := upstreamScopeFixture(t)
	catalog, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ModelsDevHTTPID, catalog, sources.ObservationMetadata{ObservedAt: at, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Revision: sources.Revision{Kind: sources.RevisionKindSourceVersion, Value: "metadata-revision"}})
	if err != nil {
		t.Fatal(err)
	}
	link := observation.Link()
	bundle, err := artifact.Build(generation)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	receipt := artifact.PublicationReceipt{SchemaVersion: artifact.PublicationReceiptSchemaVersion, RunID: "run-first", StartedAt: at, CompletedAt: at, PolicyVersion: "public-test", FreshAcquisition: true, Artifact: artifact.PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}, Sources: []artifact.PublicationSourceReceipt{{Policy: artifact.PublicationScopePolicy{Source: link.Source, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: "preserve"}, Attempt: "succeeded", EvidenceKind: "fresh", Observation: &link}}, Reviews: []evidence.ReviewCandidate{{Code: evidence.ReviewCandidateUnresolvedModelReference, ProviderID: "provider", ProviderModelID: "unresolved", SourceID: link.Source, SourceObservationID: link.ObservationID, SourceRevision: link.Revision, EvidenceChecksum: link.EvidenceChecksum, Reason: "requires a reviewed model reference"}}, ReviewObservations: []catalogs.SourceObservationLink{link}}
	publication := SourcePublication{Receipt: receipt, SourceCommit: strings.Repeat("a", 40)}
	setPublicationChecksum(t, &publication)
	return SourceRead{Changed: true, Generation: generation, PublishedAt: generation.Manifest.GeneratedAt, ChannelUpdatedAt: at, Publication: &publication, Health: HealthOK}
}

func setPublicationChecksum(t *testing.T, publication *SourcePublication) {
	t.Helper()
	raw, err := artifact.EncodePublicationReceipt(publication.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	publication.Checksum = "sha256:" + hex.EncodeToString(digest[:])
}
