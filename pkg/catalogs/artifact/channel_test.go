package artifact

import (
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

const (
	testCatalogDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testCatalogDigest2 = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testArchiveDigest  = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	testAssetSize      = 4096
)

func testAssets() []ChannelAsset {
	return []ChannelAsset{
		{Name: Filename, MediaType: MediaType, Checksum: testArchiveDigest, SizeBytes: testAssetSize},
		{Name: ChecksumFilename, MediaType: "text/plain", Checksum: testArchiveDigest, SizeBytes: testAssetSize},
	}
}

func testCandidate(digest string) Candidate {
	tag, err := ReleaseTag(digest)
	if err != nil {
		panic(err)
	}
	return Candidate{
		GenerationID:  "generation-" + digest[len(ChecksumPrefix):len(ChecksumPrefix)+8],
		Tag:           tag,
		CatalogDigest: digest,
		PublishedAt:   time.Date(2026, time.September, 2, 1, 17, 0, 0, time.UTC),
		Assets:        testAssets(),
		Verification: ReleaseVerification{
			AssetsPresent: true, ChecksumsMatch: true, AttestationVerified: true,
		},
	}
}

// TestChannelAdvancesUpdatedAtWithoutNewGeneration proves the CAT-D9 heartbeat.
// A successful verification run whose digest equals the selected digest advances
// the channel sequence and channel_updated_at alone. It creates no catalog
// generation and no immutable release.
func TestChannelAdvancesUpdatedAtWithoutNewGeneration(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, time.September, 2, 1, 17, 30, 0, time.UTC)
	candidate := testCandidate(testCatalogDigest)
	published, kind, err := Channel{}.Advance(candidate, first)
	if err != nil {
		t.Fatalf("Advance first publication: %v", err)
	}
	if kind != AdvancePromotion || !kind.CreatesGeneration() {
		t.Fatalf("first publication kind = %q, creates generation = %v", kind, kind.CreatesGeneration())
	}
	if published.Sequence != firstSequence {
		t.Fatalf("first publication sequence = %d, want %d", published.Sequence, firstSequence)
	}

	tests := []struct {
		name          string
		candidate     Candidate
		now           time.Time
		wantKind      AdvanceKind
		wantGeneraton bool
	}{
		{
			name:      "unchanged catalog advances the heartbeat",
			candidate: testCandidate(testCatalogDigest),
			now:       first.Add(4 * time.Hour),
			wantKind:  AdvanceHeartbeat,
		},
		{
			name:          "changed catalog promotes the new release",
			candidate:     testCandidate(testCatalogDigest2),
			now:           first.Add(8 * time.Hour),
			wantKind:      AdvancePromotion,
			wantGeneraton: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			next, kind, err := published.Advance(test.candidate, test.now)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if kind != test.wantKind {
				t.Fatalf("kind = %q, want %q", kind, test.wantKind)
			}
			if kind.CreatesGeneration() != test.wantGeneraton {
				t.Fatalf("creates generation = %v, want %v", kind.CreatesGeneration(), test.wantGeneraton)
			}
			if next.Sequence != published.Sequence+1 {
				t.Fatalf("sequence = %d, want %d", next.Sequence, published.Sequence+1)
			}
			if !next.ChannelUpdatedAt.Equal(test.now) {
				t.Fatalf("channel_updated_at = %s, want %s", next.ChannelUpdatedAt, test.now)
			}
			if test.wantKind != AdvanceHeartbeat {
				return
			}
			if next.GenerationID != published.GenerationID || next.Tag != published.Tag ||
				next.CatalogDigest != published.CatalogDigest {
				t.Fatalf("heartbeat changed the immutable release identity to %s %s", next.Tag, next.GenerationID)
			}
			if !next.PublishedAt.Equal(published.PublishedAt) {
				t.Fatalf("heartbeat published_at = %s, want %s", next.PublishedAt, published.PublishedAt)
			}
			if err := next.Validate(); err != nil {
				t.Fatalf("heartbeat document is invalid: %v", err)
			}
		})
	}
}

func TestChannelRejectsPromotionBeforeImmutableVerification(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 2, 5, 17, 0, 0, time.UTC)
	tests := []struct {
		name         string
		verification ReleaseVerification
	}{
		{name: "no check ran"},
		{name: "assets are missing", verification: ReleaseVerification{ChecksumsMatch: true, AttestationVerified: true}},
		{name: "checksums did not match", verification: ReleaseVerification{AssetsPresent: true, AttestationVerified: true}},
		{name: "attestation did not verify", verification: ReleaseVerification{AssetsPresent: true, ChecksumsMatch: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := testCandidate(testCatalogDigest)
			candidate.Verification = test.verification
			if _, _, err := (Channel{}).Advance(candidate, now); err == nil {
				t.Fatal("Advance accepted an unverified immutable release")
			} else if !pkgerrors.IsValidationError(err) {
				t.Fatalf("Advance error is not a validation error: %v", err)
			}
		})
	}
}

func TestChannelDocumentRoundTripsCanonicalBytes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 2, 9, 17, 0, 0, time.UTC)
	document, _, err := (Channel{}).Advance(testCandidate(testCatalogDigest), now)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	encoded, err := EncodeChannel(document)
	if err != nil {
		t.Fatalf("EncodeChannel: %v", err)
	}
	repeated, err := EncodeChannel(document)
	if err != nil {
		t.Fatalf("EncodeChannel repeat: %v", err)
	}
	if string(encoded) != string(repeated) {
		t.Fatal("EncodeChannel is not deterministic")
	}
	decoded, err := DecodeChannel(encoded)
	if err != nil {
		t.Fatalf("DecodeChannel: %v", err)
	}
	if decoded.Sequence != document.Sequence || decoded.Tag != document.Tag ||
		!decoded.ChannelUpdatedAt.Equal(document.ChannelUpdatedAt) {
		t.Fatalf("DecodeChannel returned %+v, want %+v", decoded, document)
	}
	for _, field := range []string{
		`"schema_version"`, `"channel"`, `"sequence"`, `"channel_updated_at"`,
		`"generation_id"`, `"tag"`, `"catalog_digest"`, `"published_at"`,
		`"assets"`, `"name"`, `"checksum"`,
	} {
		if !strings.Contains(string(encoded), field) {
			t.Errorf("channel document is missing %s", field)
		}
	}
}

func TestChannelRejectsNonAdvancingUpdateTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 2, 13, 17, 0, 0, time.UTC)
	document, _, err := (Channel{}).Advance(testCandidate(testCatalogDigest), now)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	for _, replay := range []time.Time{now, now.Add(-time.Hour), {}} {
		if _, _, err := document.Advance(testCandidate(testCatalogDigest), replay); err == nil {
			t.Fatalf("Advance accepted a non-advancing time %s", replay)
		}
	}
}

func TestReleaseTagNamespaceRecognizesLegacyNamespaces(t *testing.T) {
	t.Parallel()

	digest := testCatalogDigest[len(ChecksumPrefix):]
	tests := []struct {
		tag  string
		want TagNamespace
	}{
		{tag: ReleaseTagPrefix + digest, want: NamespaceCanonical},
		{tag: LegacySemanticTagPrefix + digest, want: NamespaceLegacySemantic},
		{tag: LegacyPayloadTagPrefix + digest, want: NamespaceLegacyPayload},
		{tag: ChannelName, want: NamespaceUnknown},
		{tag: "v0.14.0", want: NamespaceUnknown},
		{tag: ReleaseTagPrefix + "short", want: NamespaceUnknown},
	}
	for _, test := range tests {
		t.Run(test.tag, func(t *testing.T) {
			t.Parallel()
			if got := ReleaseTagNamespace(test.tag); got != test.want {
				t.Fatalf("ReleaseTagNamespace(%q) = %q, want %q", test.tag, got, test.want)
			}
			if got := IsReleaseTag(test.tag); got != (test.want != NamespaceUnknown) {
				t.Fatalf("IsReleaseTag(%q) = %v", test.tag, got)
			}
		})
	}
}

func TestReleaseTagAndTitleBindCanonicalNames(t *testing.T) {
	t.Parallel()

	digest := testCatalogDigest[len(ChecksumPrefix):]
	tag, err := ReleaseTag(testCatalogDigest)
	if err != nil {
		t.Fatalf("ReleaseTag: %v", err)
	}
	if tag != ReleaseTagPrefix+digest {
		t.Fatalf("ReleaseTag = %q", tag)
	}
	bare, err := ReleaseTag(digest)
	if err != nil || bare != tag {
		t.Fatalf("ReleaseTag(bare) = %q, %v", bare, err)
	}
	if _, err := ReleaseTag("sha256:not-a-digest"); err == nil {
		t.Fatal("ReleaseTag accepted a malformed digest")
	}
	title, err := ReleaseTitle("generation-1")
	if err != nil {
		t.Fatalf("ReleaseTitle: %v", err)
	}
	if title != ReleaseTitlePrefix+"generation-1" {
		t.Fatalf("ReleaseTitle = %q", title)
	}
	if _, err := ReleaseTitle("  "); err == nil {
		t.Fatal("ReleaseTitle accepted an empty generation ID")
	}
}
