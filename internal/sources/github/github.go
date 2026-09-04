// Package github observes the attested public catalog channel of one GitHub
// repository.
//
// Discovery reads the channel document from the mutable `catalog/v1` branch
// through the repository contents endpoint. The request carries the stored
// validator, so an unchanged channel costs one request and no body.
// Verification runs before
// anything becomes active. The source verifies the channel document
// attestation first. It then verifies the immutable release that the document
// selects: every named asset, its recorded checksum and size, and the build
// provenance of the archive.
//
// A document whose sequence moves backwards is a replay and the source
// rejects it. A release whose provenance fails the trust policy never becomes
// active. The release that verification last accepted stays the rollback
// target in the caller-supplied state directory.
//
// The package reads no environment variable. Every setting arrives through
// an option. Every error carries a safe reason code, which holds no URL, no
// token, and no host of a custom deployment.
package github

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// SourceName is the human-readable name of this source.
const SourceName = "Public GitHub Catalog"

// Release is one verified immutable catalog release. Every field describes
// evidence that verification already accepted.
type Release struct {
	// Tag is the immutable release tag.
	Tag string

	// GenerationID is the catalog generation the release carries.
	GenerationID string

	// CatalogDigest is the facts-only semantic digest of the catalog the
	// release carries. The publisher keys the release tag and the channel
	// document by this digest, not by the exact payload checksum that the
	// generation manifest records.
	CatalogDigest string

	// PublishedAt is the publication time the channel recorded. It is zero
	// for a release that a caller read by tag.
	PublishedAt time.Time

	// Sequence is the channel sequence that selected the release. It is zero
	// for a release that a caller read by tag.
	Sequence uint64

	// Generation is the verified immutable catalog generation.
	Generation catalogs.Generation

	// Provenance is the verified build provenance of the archive.
	Provenance attestation.Result

	// Budget reports the request budget and the request count of the cycle.
	Budget RateLimitBudget
}

// ChannelStatus is the result of one conditional channel check.
type ChannelStatus struct {
	// Changed reports whether the channel document moved since the last
	// verified read.
	Changed bool

	// ETag is the validator the reply carried.
	ETag string

	// Budget reports the request budget and the request count of the check.
	Budget RateLimitBudget
}

// Source observes the attested catalog channel of one GitHub repository.
type Source struct {
	config Config
	client *client
	state  *stateStore

	// mu serializes the durable state read and write of one refresh, so two
	// concurrent calls cannot interleave a sequence floor update.
	mu sync.Mutex
}

var _ sources.Source = (*Source)(nil)

// New builds one GitHub catalog source. It requires a state directory,
// because replay rejection and the rollback target must survive a restart.
func New(opts ...Option) (*Source, error) {
	config := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if err := config.normalize(); err != nil {
		return nil, err
	}
	restClient, err := newClient(config)
	if err != nil {
		return nil, err
	}
	store, err := newStateStore(config)
	if err != nil {
		return nil, err
	}
	return &Source{config: config, client: restClient, state: store}, nil
}

// ID returns the stable identity of this source.
func (s *Source) ID() sources.ID { return sources.ReleaseArtifactID }

// Name returns the human-friendly name of this source.
func (s *Source) Name() string { return SourceName }

// Identity returns the safe source name that fleet pacing hashes. It carries
// no URL, no token, and no host of a custom deployment.
func (s *Source) Identity() string { return SourceIdentity }

// Cleanup releases source resources. This source holds none.
func (s *Source) Cleanup() error { return nil }

// Dependencies reports the external tools this source needs. It needs none.
func (s *Source) Dependencies() []sources.Dependency { return nil }

// IsOptional reports whether a sync can succeed without this source.
func (s *Source) IsOptional() bool { return false }

// Observe verifies the release the channel selects and returns it as one
// immutable observation.
func (s *Source) Observe(ctx context.Context, _ ...sources.Option) (sources.Observation, error) {
	release, err := s.ReadChannel(ctx)
	if err != nil {
		return sources.Observation{}, err
	}
	catalog, err := catalogs.DecodeCatalogPayload(release.Generation.Payload)
	if err != nil {
		return sources.Observation{}, errors.WrapResource(
			"decode", "catalog release payload", release.GenerationID, err)
	}
	return sources.NewObservation(s.ID(), catalog, sources.ObservationMetadata{
		ObservedAt:   s.config.Now().UTC(),
		Revision:     sources.Revision{Kind: sources.RevisionKindSourceVersion, Value: release.Tag},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: len(catalog.Definitions())},
	})
}

// Changed reports whether the channel document moved since the last verified
// read. It sends one conditional request, and an unchanged channel returns no
// body. It never advances the durable validator, because only a complete
// verification may move the replay floor.
func (s *Source) Changed(ctx context.Context) (ChannelStatus, error) {
	s.mu.Lock()
	state, err := s.state.load()
	s.mu.Unlock()
	if err != nil {
		return ChannelStatus{}, err
	}
	refresh := s.client.newCycle()
	answer, _, err := refresh.channelFile(
		ctx, s.config.Channel, artifact.ChannelFilename, state.ChannelETag)
	if err != nil {
		return ChannelStatus{}, err
	}
	return ChannelStatus{
		Changed: answer.StatusCode != http.StatusNotModified,
		ETag:    answer.etag(),
		Budget:  refresh.budgetResult(),
	}, nil
}

// RollbackTarget returns the release that verification last accepted. It is
// empty before the first verified read.
func (s *Source) RollbackTarget() (ReleaseRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.state.load()
	if err != nil {
		return ReleaseRef{}, err
	}
	return state.Verified, nil
}

// ReadChannel discovers, verifies, and returns the release the channel
// selects. It advances the durable validator, the replay floor, and the
// rollback target only after every check passes.
func (s *Source) ReadChannel(ctx context.Context) (Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.state.load()
	if err != nil {
		return Release{}, err
	}
	refresh := s.client.newCycle()
	answer, body, err := refresh.channelFile(ctx, s.config.Channel, artifact.ChannelFilename, "")
	if err != nil {
		return Release{}, err
	}
	document, err := s.readChannelDocument(ctx, refresh, body)
	if err != nil {
		return Release{}, err
	}
	if document.Sequence < state.Sequence {
		return Release{}, &errors.ConflictError{
			Resource: "catalog channel document",
			Expected: sequenceLabel(state.Sequence),
			Actual:   sequenceLabel(document.Sequence),
			Message:  "the channel sequence moved backwards",
		}
	}
	release, err := s.readRelease(ctx, refresh, document.Tag, document.Assets)
	if err != nil {
		return Release{}, err
	}
	release.PublishedAt = document.PublishedAt
	release.Sequence = document.Sequence
	if release.GenerationID != document.GenerationID {
		return Release{}, sourceValidation("release.generation_id", release.GenerationID,
			"does not match the generation the channel selects")
	}
	if trimChecksum(release.CatalogDigest) != trimChecksum(document.CatalogDigest) {
		return Release{}, sourceValidation("release.catalog_digest", release.Tag,
			"does not match the catalog digest the channel selects")
	}

	now := s.config.Now().UTC()
	if err := s.state.save(State{
		Repository:  s.config.Repository,
		Channel:     s.config.Channel,
		ChannelETag: answer.etag(),
		Sequence:    document.Sequence,
		Verified: ReleaseRef{
			Tag:           release.Tag,
			GenerationID:  release.GenerationID,
			CatalogDigest: release.CatalogDigest,
			VerifiedAt:    now,
		},
		UpdatedAt: now,
	}); err != nil {
		return Release{}, err
	}
	return release, nil
}

// ReadRelease reads and verifies one immutable release by tag. It accepts the
// canonical namespace and both retired namespaces, so a rollback target and a
// legacy release stay readable. It never advances the durable state, because
// a read by tag is an explicit override and not a discovery.
func (s *Source) ReadRelease(ctx context.Context, tag string) (Release, error) {
	if !artifact.IsReleaseTag(tag) {
		return Release{}, sourceValidation("release.tag", tag,
			"does not name an immutable catalog release")
	}
	return s.readRelease(ctx, s.client.newCycle(), tag, nil)
}

// readChannelDocument verifies the provenance that signs the channel document
// bytes and then decodes them. It verifies before it decodes, so unverified
// bytes never reach the channel schema.
func (s *Source) readChannelDocument(
	ctx context.Context,
	refresh *cycle,
	body []byte,
) (artifact.Channel, error) {
	if _, err := refresh.verifyBytes(ctx, body); err != nil {
		return artifact.Channel{}, errors.WrapResource(
			"verify", "catalog channel document", s.config.Channel, err)
	}
	return artifact.DecodeChannel(body)
}

// readRelease downloads and verifies one immutable release. A non-empty
// expected list binds every asset to the checksum and size the channel
// records. An empty list reads the release by tag alone, which is the
// rollback and legacy path.
func (s *Source) readRelease(
	ctx context.Context,
	refresh *cycle,
	tag string,
	expected []artifact.ChannelAsset,
) (Release, error) {
	_, document, err := refresh.releaseByTag(ctx, tag, "")
	if err != nil {
		return Release{}, err
	}
	assets, err := s.downloadAssets(ctx, refresh, document, expected)
	if err != nil {
		return Release{}, err
	}
	verifier := &provenanceVerifier{cycle: refresh}
	generation, err := artifact.VerifyRelease(ctx, artifact.Release{
		Archive:     assets[artifact.Filename],
		Checksum:    assets[artifact.ChecksumFilename],
		Attestation: assets[artifact.AttestationFilename],
	}, verifier)
	if err != nil {
		return Release{}, err
	}
	digest, err := generation.SemanticChecksum()
	if err != nil {
		return Release{}, err
	}
	return Release{
		Tag:           tag,
		GenerationID:  generation.Manifest.GenerationID,
		CatalogDigest: digest,
		Generation:    generation,
		Provenance:    verifier.result,
		Budget:        refresh.budgetResult(),
	}, nil
}

// downloadAssets reads every asset the release must carry. It checks the
// channel-recorded size and checksum of each named asset before any byte
// reaches the artifact reader.
func (s *Source) downloadAssets(
	ctx context.Context,
	refresh *cycle,
	document releaseDocument,
	expected []artifact.ChannelAsset,
) (map[string][]byte, error) {
	wanted := releaseAssetNames(expected)
	downloaded := make(map[string][]byte, len(wanted))
	recorded := make(map[string]artifact.ChannelAsset, len(expected))
	for _, asset := range expected {
		recorded[asset.Name] = asset
	}
	for _, name := range wanted {
		asset, err := document.asset(name)
		if err != nil {
			return nil, err
		}
		if record, found := recorded[name]; found && record.SizeBytes != asset.Size {
			return nil, sourceValidation("release.asset.size_bytes", name,
				"does not match the size the channel records")
		}
		body, err := refresh.assetBytes(ctx, asset)
		if err != nil {
			return nil, err
		}
		if record, found := recorded[name]; found {
			if digestHex(body) != trimChecksum(record.Checksum) {
				return nil, sourceValidation("release.asset.checksum", name,
					"does not match the checksum the channel records")
			}
		}
		downloaded[name] = body
	}
	for _, name := range requiredAssets() {
		if len(downloaded[name]) == 0 {
			return nil, errors.NewNotFoundError("catalog release asset", name)
		}
	}
	return downloaded, nil
}

// requiredAssets names the three assets every immutable release must carry.
func requiredAssets() []string {
	return []string{artifact.Filename, artifact.ChecksumFilename, artifact.AttestationFilename}
}

// releaseAssetNames returns the asset names to download. It always covers the
// three required assets and adds every other asset the channel records.
func releaseAssetNames(expected []artifact.ChannelAsset) []string {
	names := requiredAssets()
	seen := make(map[string]struct{}, len(names)+len(expected))
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, asset := range expected {
		if _, found := seen[asset.Name]; found {
			continue
		}
		seen[asset.Name] = struct{}{}
		names = append(names, asset.Name)
	}
	return names
}

// trimChecksum removes the algorithm prefix from a recorded checksum.
func trimChecksum(checksum string) string {
	return strings.TrimPrefix(strings.TrimSpace(checksum), artifact.ChecksumPrefix)
}

// sequenceLabel renders one channel sequence for a conflict report.
func sequenceLabel(sequence uint64) string {
	return "sequence " + strconv.FormatUint(sequence, 10)
}
