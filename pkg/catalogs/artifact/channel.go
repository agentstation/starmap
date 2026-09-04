package artifact

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// ChannelName is the stable public discovery release that carries the
	// channel document. It is a mutable pointer, not an immutable release.
	ChannelName = "catalog-latest"
	// ChannelFilename is the attested channel document asset name.
	ChannelFilename = "catalog-latest.json"
	// ChannelMediaType is the media type of the channel document.
	ChannelMediaType = "application/vnd.agentstation.starmap.catalog-channel.v1+json"
	// ChannelSchemaVersion is the current channel document schema version.
	ChannelSchemaVersion uint64 = 1
	// ChannelTitle is the title of the channel release.
	ChannelTitle = "Catalog latest"

	// ReleaseTagPrefix starts the canonical immutable release tag.
	ReleaseTagPrefix = "catalog-"
	// LegacySemanticTagPrefix starts the retired facts-digest namespace.
	LegacySemanticTagPrefix = "catalog-semantic-"
	// LegacyPayloadTagPrefix starts the first retired payload namespace.
	LegacyPayloadTagPrefix = "catalog-payload-"
	// ReleaseTitlePrefix starts the immutable release title.
	ReleaseTitlePrefix = "Catalog "

	// ChecksumPrefix starts every checksum this package reports.
	ChecksumPrefix = "sha256:"

	digestHexLength   = 64
	maxChannelBytes   = 1 << 20
	channelIndent     = "  "
	channelTrailer    = "\n"
	minChannelAssets  = 1
	firstSequence     = 1
	maxChannelAssets  = 32
	maxChannelIDBytes = 512
)

// TagNamespace names the publication namespace of one catalog release tag.
type TagNamespace string

const (
	// NamespaceCanonical is the current `catalog-<catalog-digest>` namespace.
	NamespaceCanonical TagNamespace = "canonical"
	// NamespaceLegacySemantic is the retired `catalog-semantic-*` namespace.
	NamespaceLegacySemantic TagNamespace = "legacy-semantic"
	// NamespaceLegacyPayload is the retired `catalog-payload-*` namespace.
	NamespaceLegacyPayload TagNamespace = "legacy-payload"
	// NamespaceUnknown names a tag that no catalog namespace owns.
	NamespaceUnknown TagNamespace = "unknown"
)

// ReleaseTagNamespace reports the namespace that owns one release tag. It
// recognizes the canonical namespace and both retired namespaces, so rollback
// and readback keep every historical immutable release. The channel pointer is
// not an immutable release and belongs to no namespace.
func ReleaseTagNamespace(tag string) TagNamespace {
	trimmed := strings.TrimSpace(tag)
	switch {
	case trimmed == ChannelName:
		return NamespaceUnknown
	case isDigestTag(trimmed, LegacySemanticTagPrefix):
		return NamespaceLegacySemantic
	case isDigestTag(trimmed, LegacyPayloadTagPrefix):
		return NamespaceLegacyPayload
	case isDigestTag(trimmed, ReleaseTagPrefix):
		return NamespaceCanonical
	default:
		return NamespaceUnknown
	}
}

// IsReleaseTag reports whether one tag names an immutable catalog release in
// any recognized namespace.
func IsReleaseTag(tag string) bool {
	return ReleaseTagNamespace(tag) != NamespaceUnknown
}

// ReleaseTag returns the canonical immutable release tag for one catalog
// digest. It accepts a prefixed or bare SHA-256 hex digest.
func ReleaseTag(catalogDigest string) (string, error) {
	digest, err := digestHex(catalogDigest)
	if err != nil {
		return "", err
	}
	return ReleaseTagPrefix + digest, nil
}

// ReleaseTitle returns the immutable release title for one generation ID.
func ReleaseTitle(generationID string) (string, error) {
	trimmed := strings.TrimSpace(generationID)
	if trimmed == "" || len(trimmed) > maxChannelIDBytes {
		return "", channelValidation("release_title", generationID, "generation ID is required")
	}
	return ReleaseTitlePrefix + trimmed, nil
}

// ChannelAsset binds one published release asset name to its exact checksum.
type ChannelAsset struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Checksum  string `json:"checksum"`
	SizeBytes int64  `json:"size_bytes"`
}

// Channel is the mutable attested discovery document that selects one
// immutable catalog release. The publisher advances its sequence and
// `channel_updated_at` after every successful verification run. The immutable
// identity fields change only when the publisher promotes a new release.
type Channel struct {
	SchemaVersion    uint64         `json:"schema_version"`
	Name             string         `json:"channel"`
	Sequence         uint64         `json:"sequence"`
	ChannelUpdatedAt time.Time      `json:"channel_updated_at"`
	GenerationID     string         `json:"generation_id"`
	Tag              string         `json:"tag"`
	CatalogDigest    string         `json:"catalog_digest"`
	PublishedAt      time.Time      `json:"published_at"`
	Assets           []ChannelAsset `json:"assets"`
}

// ReleaseVerification records the immutable release checks that the publisher
// completed. The channel selects a release only after every check passes, so
// verification always precedes promotion.
type ReleaseVerification struct {
	AssetsPresent       bool `json:"assets_present"`
	ChecksumsMatch      bool `json:"checksums_match"`
	AttestationVerified bool `json:"attestation_verified"`
}

// Complete reports whether every immutable release check passed.
func (v ReleaseVerification) Complete() bool {
	return v.AssetsPresent && v.ChecksumsMatch && v.AttestationVerified
}

// Candidate is one immutable catalog release offered to the channel.
type Candidate struct {
	GenerationID  string
	Tag           string
	CatalogDigest string
	PublishedAt   time.Time
	Assets        []ChannelAsset
	Verification  ReleaseVerification
}

// AdvanceKind names the outcome of one successful publisher run.
type AdvanceKind string

const (
	// AdvanceHeartbeat keeps the selected immutable release and advances only
	// the channel sequence and `channel_updated_at`.
	AdvanceHeartbeat AdvanceKind = "heartbeat"
	// AdvancePromotion selects a different verified immutable release.
	AdvancePromotion AdvanceKind = "promotion"
)

// CreatesGeneration reports whether the outcome selects a new catalog
// generation. A heartbeat creates no generation and no immutable release.
func (k AdvanceKind) CreatesGeneration() bool {
	return k == AdvancePromotion
}

// Advance returns the channel document that follows one successful publisher
// run. A candidate whose digest equals the selected digest advances the
// sequence and `channel_updated_at` alone, so an unchanged catalog creates no
// generation and no immutable release. A different verified digest promotes
// that release. An incomplete verification returns a typed validation error.
func (c Channel) Advance(candidate Candidate, now time.Time) (Channel, AdvanceKind, error) {
	if err := candidate.Validate(); err != nil {
		return Channel{}, "", err
	}
	if !candidate.Verification.Complete() {
		return Channel{}, "", channelValidation(
			"candidate.verification",
			candidate.Tag,
			"the channel cannot select an immutable release before its assets, checksums, and attestation verify",
		)
	}
	if now.IsZero() {
		return Channel{}, "", channelValidation("channel_updated_at", now, "is required")
	}
	if c.Sequence == math.MaxUint64 {
		return Channel{}, "", channelValidation("sequence", c.Sequence, "has reached its maximum value")
	}
	if c.Sequence > 0 {
		if err := c.Validate(); err != nil {
			return Channel{}, "", err
		}
		if !now.After(c.ChannelUpdatedAt) {
			return Channel{}, "", channelValidation(
				"channel_updated_at",
				now,
				"must advance beyond the current channel document time",
			)
		}
	}

	next := Channel{
		SchemaVersion:    ChannelSchemaVersion,
		Name:             ChannelName,
		Sequence:         c.Sequence + 1,
		ChannelUpdatedAt: now.UTC(),
		GenerationID:     c.GenerationID,
		Tag:              c.Tag,
		CatalogDigest:    c.CatalogDigest,
		PublishedAt:      c.PublishedAt.UTC(),
		Assets:           copyChannelAssets(c.Assets),
	}
	if c.Sequence > 0 && c.CatalogDigest == candidate.CatalogDigest {
		return next, AdvanceHeartbeat, nil
	}
	next.GenerationID = candidate.GenerationID
	next.Tag = candidate.Tag
	next.CatalogDigest = candidate.CatalogDigest
	next.PublishedAt = candidate.PublishedAt.UTC()
	next.Assets = copyChannelAssets(candidate.Assets)
	return next, AdvancePromotion, nil
}

// Validate reports whether the channel document is internally consistent.
func (c Channel) Validate() error {
	if c.SchemaVersion != ChannelSchemaVersion {
		return channelValidation("schema_version", c.SchemaVersion, "is not the current channel schema version")
	}
	if c.Name != ChannelName {
		return channelValidation("channel", c.Name, "is not the canonical channel name")
	}
	if c.Sequence < firstSequence {
		return channelValidation("sequence", c.Sequence, "must start at one")
	}
	if c.ChannelUpdatedAt.IsZero() {
		return channelValidation("channel_updated_at", c.ChannelUpdatedAt, "is required")
	}
	if c.PublishedAt.IsZero() {
		return channelValidation("published_at", c.PublishedAt, "is required")
	}
	return validateReleaseIdentity(c.GenerationID, c.Tag, c.CatalogDigest, c.Assets)
}

// Validate reports whether the candidate names a complete immutable release.
func (c Candidate) Validate() error {
	if c.PublishedAt.IsZero() {
		return channelValidation("candidate.published_at", c.PublishedAt, "is required")
	}
	return validateReleaseIdentity(c.GenerationID, c.Tag, c.CatalogDigest, c.Assets)
}

// EncodeChannel renders one channel document as canonical indented JSON. Equal
// documents always encode to equal bytes.
func EncodeChannel(document Channel) ([]byte, error) {
	if err := document.Validate(); err != nil {
		return nil, err
	}
	canonical := document
	canonical.ChannelUpdatedAt = document.ChannelUpdatedAt.UTC()
	canonical.PublishedAt = document.PublishedAt.UTC()
	canonical.Assets = copyChannelAssets(document.Assets)
	sort.Slice(canonical.Assets, func(first, second int) bool {
		return canonical.Assets[first].Name < canonical.Assets[second].Name
	})
	data, err := json.MarshalIndent(canonical, "", channelIndent)
	if err != nil {
		return nil, channelValidation("document", document.Tag, err.Error())
	}
	return append(data, channelTrailer...), nil
}

// DecodeChannel reads and validates one channel document.
func DecodeChannel(data []byte) (Channel, error) {
	if len(data) == 0 {
		return Channel{}, channelValidation("document", len(data), "is empty")
	}
	if len(data) > maxChannelBytes {
		return Channel{}, channelValidation("document", len(data), "exceeds the channel document size limit")
	}
	var document Channel
	if err := decodeStrictJSON(data, &document); err != nil {
		return Channel{}, err
	}
	if err := document.Validate(); err != nil {
		return Channel{}, err
	}
	return document, nil
}

func validateReleaseIdentity(generationID, tag, catalogDigest string, assets []ChannelAsset) error {
	if strings.TrimSpace(generationID) == "" || len(generationID) > maxChannelIDBytes {
		return channelValidation("generation_id", generationID, "is required")
	}
	digest, err := digestHex(catalogDigest)
	if err != nil {
		return err
	}
	if ReleaseTagNamespace(tag) == NamespaceUnknown {
		return channelValidation("tag", tag, "does not name an immutable catalog release")
	}
	if ReleaseTagNamespace(tag) == NamespaceCanonical && tag != ReleaseTagPrefix+digest {
		return channelValidation("tag", tag, "does not bind the catalog digest")
	}
	if len(assets) < minChannelAssets || len(assets) > maxChannelAssets {
		return channelValidation("assets", len(assets), "must name every published release asset")
	}
	seen := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if strings.TrimSpace(asset.Name) == "" {
			return channelValidation("assets.name", asset.Name, "is required")
		}
		if _, duplicate := seen[asset.Name]; duplicate {
			return channelValidation("assets.name", asset.Name, "is repeated")
		}
		seen[asset.Name] = struct{}{}
		if _, err := digestHex(asset.Checksum); err != nil {
			return channelValidation("assets.checksum", asset.Name, "is not a SHA-256 checksum")
		}
		if asset.SizeBytes <= 0 {
			return channelValidation("assets.size_bytes", asset.Name, "must be positive")
		}
	}
	return nil
}

func digestHex(value string) (string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), ChecksumPrefix)
	if !isDigestHex(trimmed) {
		return "", channelValidation("catalog_digest", value, "is not a SHA-256 hex digest")
	}
	return trimmed, nil
}

func isDigestTag(tag, prefix string) bool {
	return strings.HasPrefix(tag, prefix) && isDigestHex(strings.TrimPrefix(tag, prefix))
}

func isDigestHex(value string) bool {
	if len(value) != digestHexLength {
		return false
	}
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'f':
		default:
			return false
		}
	}
	return true
}

func copyChannelAssets(assets []ChannelAsset) []ChannelAsset {
	if len(assets) == 0 {
		return nil
	}
	copied := make([]ChannelAsset, len(assets))
	copy(copied, assets)
	return copied
}

func channelValidation(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "catalog_channel." + field,
		Value:   value,
		Message: message,
	}
}
