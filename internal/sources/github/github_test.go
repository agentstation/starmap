package github

import (
	stderrors "errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// requestsPerChangedCycle is the request budget of one cycle that promotes a
// release. The cycle reads the channel document from the channel branch and
// the channel provenance. It then reads the immutable release, its three
// assets, and the archive provenance.
const requestsPerChangedCycle = 7

// requestsPerUnchangedCycle is the request budget of one cycle that finds an
// unchanged channel.
const requestsPerUnchangedCycle = 1

// requestsPerChannelRead is the request budget a cycle spends before it
// reaches the immutable release: the channel document and its provenance.
const requestsPerChannelRead = 2

// progressRecorder collects transfer progress reports.
type progressRecorder struct {
	mu     sync.Mutex
	stages map[remote.TransferStage]int
	bytes  int64
}

func newProgressRecorder() *progressRecorder {
	return &progressRecorder{stages: make(map[remote.TransferStage]int)}
}

func (p *progressRecorder) record() remote.ProgressFunc {
	return func(progress remote.TransferProgress) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.stages[progress.Stage]++
		if progress.Stage == remote.TransferStageComplete {
			p.bytes += progress.BytesReceived
		}
	}
}

func (p *progressRecorder) count(stage remote.TransferStage) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stages[stage]
}

func (p *progressRecorder) received() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bytes
}

func TestGitHubSourceVerifiesTheChannelBranch(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	published := publishCatalog(t, server, "channel-branch-generation", testChannelSequence)
	attester := &recordingAttester{}
	progress := newProgressRecorder()
	source := newTestSource(t, server,
		WithAttester(attester.attest()),
		WithProgress(progress.record()),
	)

	observation, err := source.Observe(t.Context())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.SourceID != sources.ReleaseArtifactID {
		t.Fatalf("source = %q, want %q", observation.SourceID, sources.ReleaseArtifactID)
	}
	if observation.Revision.Kind != sources.RevisionKindSourceVersion ||
		observation.Revision.Value != published.Tag {
		t.Fatalf("revision = %+v, want the immutable release tag %s",
			observation.Revision, published.Tag)
	}
	if observation.Catalog == nil {
		t.Fatal("observation carries no catalog")
	}
	if observation.Status != sources.ObservationStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", observation.Status)
	}

	// Verification covers the channel document and the release archive.
	wantDigests := []string{hexDigest(published.ChannelDoc), hexDigest(published.Assets[0].Body)}
	if got := attester.digests(); len(got) != len(wantDigests) ||
		got[0] != wantDigests[0] || got[1] != wantDigests[1] {
		t.Fatalf("verified digests = %v, want %v", got, wantDigests)
	}
	assertPolicy(t, attester.calls[0].Policy)
	assertPolicy(t, attester.calls[1].Policy)

	if got := server.requestCount(); got != requestsPerChangedCycle {
		t.Fatalf("requests = %d, want %d", got, requestsPerChangedCycle)
	}
	if progress.count(remote.TransferStageComplete) != requestsPerChangedCycle {
		t.Fatalf("completed transfers = %d, want %d",
			progress.count(remote.TransferStageComplete), requestsPerChangedCycle)
	}
	if progress.received() <= 0 {
		t.Fatal("progress reported no received bytes")
	}

	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if target.Tag != published.Tag || target.GenerationID != published.Channel.GenerationID {
		t.Fatalf("rollback target = %+v, want %s", target, published.Tag)
	}

	// The verified read stored the validator, so the next check is conditional.
	before := server.requestCount()
	status, err := source.Changed(t.Context())
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}
	if status.Changed {
		t.Fatal("Changed reported a moved channel after a verified read")
	}
	if got := server.requestCount() - before; got != requestsPerUnchangedCycle {
		t.Fatalf("unchanged cycle requests = %d, want %d", got, requestsPerUnchangedCycle)
	}
}

// assertPolicy checks the trust policy that every verification receives.
func assertPolicy(t *testing.T, policy attestation.Policy) {
	t.Helper()
	if policy.Repository != testRepository {
		t.Errorf("policy repository = %q, want %q", policy.Repository, testRepository)
	}
	if policy.Workflow != DefaultSignerWorkflow {
		t.Errorf("policy workflow = %q, want %q", policy.Workflow, DefaultSignerWorkflow)
	}
	if policy.Issuer != attestation.GitHubOIDCIssuer {
		t.Errorf("policy issuer = %q, want %q", policy.Issuer, attestation.GitHubOIDCIssuer)
	}
	if policy.PredicateType != attestation.BuildProvenancePredicateType {
		t.Errorf("policy predicate = %q, want %q",
			policy.PredicateType, attestation.BuildProvenancePredicateType)
	}
	if !policy.DenySelfHostedRunners {
		t.Error("policy accepts a self-hosted runner")
	}
	if len(policy.TrustedRootJSON) != len(attestation.DefaultTrustedRootJSON()) {
		t.Error("policy does not carry the compiled Sigstore trusted root")
	}
}

func TestGitHubSourceRejectsReplayedChannelDocuments(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	current := publishCatalog(t, server, "current-generation", testChannelSequence)
	attester := &recordingAttester{}
	source := newTestSource(t, server, WithAttester(attester.attest()))
	if _, err := source.ReadChannel(t.Context()); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}

	// Republish an older sequence that selects a different release.
	publishCatalog(t, server, "older-generation", testChannelSequence-1)
	server.setChannelETag(testETag + "-moved")

	_, err := source.ReadChannel(t.Context())
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("error = %v, want *errors.ConflictError", err)
	}
	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if target.Tag != current.Tag {
		t.Fatalf("rollback target = %q, want the last verified release %q", target.Tag, current.Tag)
	}
}

func TestGitHubSourceRejectsTrustNegativeReleases(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	trusted := publishCatalog(t, server, "trusted-generation", testChannelSequence)
	attester := &recordingAttester{}
	source := newTestSource(t, server, WithAttester(attester.attest()))
	if _, err := source.ReadChannel(t.Context()); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}

	// The publisher promotes a release whose provenance fails the policy.
	promoted := publishCatalog(t, server, "untrusted-generation", testChannelSequence+1)
	server.setChannelETag(testETag + "-moved")
	attester.reject = map[string]error{
		hexDigest(promoted.Assets[0].Body): &attestation.TrustError{
			Stage:   "signature",
			Message: "the bundle does not satisfy the Starmap policy",
		},
	}

	_, err := source.ReadChannel(t.Context())
	var trust *attestation.TrustError
	if !stderrors.As(err, &trust) {
		t.Fatalf("error = %v, want *attestation.TrustError", err)
	}
	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if target.Tag != trusted.Tag {
		t.Fatalf("rollback target = %q, want the last verified release %q", target.Tag, trusted.Tag)
	}
}

func TestGitHubSourceRejectsUnattestedChannelDocuments(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	published := publishCatalog(t, server, "unattested-generation", testChannelSequence)
	attester := &recordingAttester{
		reject: map[string]error{
			hexDigest(published.ChannelDoc): &attestation.TrustError{
				Stage:   "policy",
				Message: "the channel document carries no accepted provenance",
			},
		},
	}
	source := newTestSource(t, server, WithAttester(attester.attest()))

	_, err := source.ReadChannel(t.Context())
	var trust *attestation.TrustError
	if !stderrors.As(err, &trust) {
		t.Fatalf("error = %v, want *attestation.TrustError", err)
	}
	// The source stops at the channel document and never reaches the release.
	if got := server.requestCount(); got != requestsPerChannelRead {
		t.Fatalf("requests = %d, want %d", got, requestsPerChannelRead)
	}
	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if !target.Empty() {
		t.Fatalf("rollback target = %+v, want no verified release", target)
	}
}

func TestGitHubSourceRejectsAssetsThatMissTheChannelRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		corrupt func(asset *artifact.ChannelAsset)
	}{
		{
			name: "recorded checksum",
			corrupt: func(asset *artifact.ChannelAsset) {
				asset.Checksum = artifact.ChecksumPrefix + hexDigest([]byte("other bytes"))
			},
		},
		{
			name:    "recorded size",
			corrupt: func(asset *artifact.ChannelAsset) { asset.SizeBytes++ },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := newFixtureServer(t)
			published := publishCatalog(t, server, "mismatched-generation", testChannelSequence)
			document := published.Channel
			document.Assets = append([]artifact.ChannelAsset(nil), document.Assets...)
			for index := range document.Assets {
				if document.Assets[index].Name == artifact.Filename {
					test.corrupt(&document.Assets[index])
				}
			}
			encoded, err := artifact.EncodeChannel(document)
			if err != nil {
				t.Fatalf("encode channel: %v", err)
			}
			server.publishChannel(encoded)
			attester := &recordingAttester{}
			source := newTestSource(t, server, WithAttester(attester.attest()))

			_, err = source.ReadChannel(t.Context())
			var invalid *errors.ValidationError
			if !stderrors.As(err, &invalid) {
				t.Fatalf("error = %v, want *errors.ValidationError", err)
			}
			target, err := source.RollbackTarget()
			if err != nil {
				t.Fatalf("RollbackTarget: %v", err)
			}
			if !target.Empty() {
				t.Fatalf("rollback target = %+v, want no verified release for %s",
					target, published.Tag)
			}
		})
	}
}

func TestGitHubSourceRejectsUnsignedEvidenceWithTheDefaultEngine(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	published := publishCatalog(t, server, "unsigned-generation", testChannelSequence)
	// Serve real published provenance that binds a different artifact.
	server.setBundle(hexDigest(published.ChannelDoc), readFixture(t, committedBundle))
	source := newTestSource(t, server)

	if _, err := source.ReadChannel(t.Context()); err == nil {
		t.Fatal("ReadChannel accepted provenance that binds another artifact")
	}
	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if !target.Empty() {
		t.Fatalf("rollback target = %+v, want no verified release", target)
	}
}

func TestGitHubSourcePolicyAcceptsPublishedCatalogProvenance(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	source := newTestSource(t, server)
	result, err := source.config.Attester(
		t.Context(),
		readFixture(t, committedBundle),
		committedArtifactDigest,
		source.config.policy(),
	)
	if err != nil {
		t.Fatalf("verify published provenance: %v", err)
	}
	if result.RunnerEnvironment != attestation.HostedRunnerEnvironment {
		t.Fatalf("runner = %q, want %q", result.RunnerEnvironment, attestation.HostedRunnerEnvironment)
	}
	if result.SourceRepositoryURI != "https://github.com/"+testRepository {
		t.Fatalf("repository = %q, want the configured publisher", result.SourceRepositoryURI)
	}
}

func TestGitHubSourceReadsLegacyReleaseTags(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	attester := &recordingAttester{}
	source := newTestSource(t, server, WithAttester(attester.attest()))

	tests := []struct {
		name   string
		prefix string
		id     string
	}{
		{"legacy semantic", artifact.LegacySemanticTagPrefix, "legacy-semantic-generation"},
		{"legacy payload", artifact.LegacyPayloadTagPrefix, "legacy-payload-generation"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generation := testGeneration(t, test.id)
			assets := releaseAssetsOf(t, generation)
			digest := hexDigest(assets[0].Body)
			tag := legacyTag(test.prefix, digest)
			if artifact.ReleaseTagNamespace(tag) == artifact.NamespaceUnknown {
				t.Fatalf("tag %q names no catalog release namespace", tag)
			}
			server.publish(releaseFixture{Tag: tag, Assets: assets})

			release, err := source.ReadRelease(t.Context(), tag)
			if err != nil {
				t.Fatalf("ReadRelease %s: %v", tag, err)
			}
			if release.Tag != tag || release.GenerationID != test.id {
				t.Fatalf("release = %+v, want %s / %s", release, tag, test.id)
			}
			if release.Provenance.PredicateType != attestation.BuildProvenancePredicateType {
				t.Fatalf("provenance predicate = %q, want the build provenance type",
					release.Provenance.PredicateType)
			}
		})
	}

	// A tag outside every catalog namespace is not a release.
	if _, err := source.ReadRelease(t.Context(), "v1.2.3"); err == nil {
		t.Fatal("ReadRelease accepted a tag outside every catalog namespace")
	}
	// A read by tag is an explicit override and moves no durable state.
	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if !target.Empty() {
		t.Fatalf("rollback target = %+v, want no verified release", target)
	}
}

func TestGitHubSourceReportsRateLimitBudget(t *testing.T) {
	t.Parallel()

	reset := time.Date(2026, time.September, 2, 13, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		limit       int
		used        int
		wantPercent int
		wantWarn    bool
	}{
		{name: "inside the budget", limit: 5000, used: 500, wantPercent: 10, wantWarn: false},
		{name: "at the warning threshold", limit: 5000, used: 4000, wantPercent: 80, wantWarn: true},
		{name: "beyond the warning threshold", limit: 5000, used: 4900, wantPercent: 98, wantWarn: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := newFixtureServer(t)
			publishCatalog(t, server, "budget-generation", testChannelSequence)
			server.setRateHeaders(map[string]string{
				headerRateLimit:     strconv.Itoa(test.limit),
				headerRateUsed:      strconv.Itoa(test.used),
				headerRateRemaining: strconv.Itoa(test.limit - test.used),
				headerRateReset:     strconv.FormatInt(reset.Unix(), 10),
			})
			attester := &recordingAttester{}
			source := newTestSource(t, server, WithAttester(attester.attest()))

			release, err := source.ReadChannel(t.Context())
			if err != nil {
				t.Fatalf("ReadChannel: %v", err)
			}
			budget := release.Budget
			if !budget.Observed {
				t.Fatal("the source reported no rate-limit budget")
			}
			if budget.Limit != test.limit || budget.Used != test.used ||
				budget.Remaining != test.limit-test.used {
				t.Fatalf("budget = %+v, want limit %d and used %d", budget, test.limit, test.used)
			}
			if !budget.ResetAt.Equal(reset) {
				t.Fatalf("reset = %s, want %s", budget.ResetAt, reset)
			}
			if budget.UsedPercent() != test.wantPercent {
				t.Fatalf("used percent = %d, want %d", budget.UsedPercent(), test.wantPercent)
			}
			if budget.Warn() != test.wantWarn {
				t.Fatalf("warn = %t, want %t at %d percent", budget.Warn(), test.wantWarn, test.wantPercent)
			}
			if budget.Requests != requestsPerChangedCycle {
				t.Fatalf("requests this cycle = %d, want %d",
					budget.Requests, requestsPerChangedCycle)
			}

			status, err := source.Changed(t.Context())
			if err != nil {
				t.Fatalf("Changed: %v", err)
			}
			if status.Budget.Requests != requestsPerUnchangedCycle {
				t.Fatalf("unchanged cycle requests = %d, want %d",
					status.Budget.Requests, requestsPerUnchangedCycle)
			}
		})
	}
}

func TestGitHubSourceRejectsUnusableConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []Option
	}{
		{"no state directory", []Option{WithRepository(testRepository)}},
		{"empty repository", []Option{WithStateDirectory(t.TempDir()), WithRepository("starmap")}},
		{"empty channel", []Option{WithStateDirectory(t.TempDir()), WithChannel("  ")}},
		{"empty signer workflow", []Option{WithStateDirectory(t.TempDir()), WithSignerWorkflow("")}},
		{"empty trusted root", []Option{WithStateDirectory(t.TempDir()), WithTrustedRoot(nil)}},
		{
			"unusable transfer policy",
			[]Option{WithStateDirectory(t.TempDir()), WithTransferPolicy(remote.TransferPolicy{})},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(test.opts...); err == nil {
				t.Fatalf("New accepted %s", test.name)
			}
		})
	}
}

func TestGitHubSourceDeclaresItsContract(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	source := newTestSource(t, server)
	if source.ID() != sources.ReleaseArtifactID {
		t.Fatalf("ID = %q, want %q", source.ID(), sources.ReleaseArtifactID)
	}
	if source.Identity() != SourceIdentity {
		t.Fatalf("Identity = %q, want %q", source.Identity(), SourceIdentity)
	}
	if source.IsOptional() {
		t.Fatal("the verified catalog source reports itself optional")
	}
	if len(source.Dependencies()) != 0 {
		t.Fatalf("dependencies = %v, want none", source.Dependencies())
	}
	if err := source.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

// The publisher keys the release tag and the channel catalog digest by the
// facts-only semantic checksum. The exact payload checksum in the generation
// manifest differs from it. A consumer that compares the payload checksum to
// the channel therefore rejects every published release.
func TestGitHubSourceReportsTheSemanticCatalogDigest(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	published := publishCatalog(t, server, "semantic-digest-generation", testChannelSequence)
	attester := &recordingAttester{}
	source := newTestSource(t, server, WithAttester(attester.attest()))

	want := semanticChecksum(t, published.Generation)
	if want == published.Generation.Manifest.Payload.Checksum {
		t.Fatal("fixture semantic checksum equals the payload checksum, the test proves nothing")
	}
	if artifact.ChecksumPrefix+published.Channel.CatalogDigest != want {
		t.Fatalf("channel catalog digest = %s, want the semantic checksum %s",
			published.Channel.CatalogDigest, want)
	}

	release, err := source.ReadChannel(t.Context())
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if release.CatalogDigest != want {
		t.Fatalf("release catalog digest = %s, want %s", release.CatalogDigest, want)
	}
	if release.Tag != published.Tag {
		t.Fatalf("release tag = %s, want %s", release.Tag, published.Tag)
	}

	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if target.CatalogDigest != want {
		t.Fatalf("rollback target digest = %s, want %s", target.CatalogDigest, want)
	}
}

func TestGitHubSourceSkipsTheBodyWhenTheChannelIsUnchanged(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	publishCatalog(t, server, "unchanged-generation", testChannelSequence)
	attester := &recordingAttester{}
	source := newTestSource(t, server, WithAttester(attester.attest()))

	if _, err := source.ReadChannel(t.Context()); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if got := server.channelBodyCount(); got != 1 {
		t.Fatalf("channel document bodies = %d, want 1 after the first read", got)
	}

	// The stored validator makes the next check conditional. A 304 answers
	// with no body, so the poll spends one request and no transfer.
	before := server.requestCount()
	status, err := source.Changed(t.Context())
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}
	if status.Changed {
		t.Fatal("Changed reported a moved channel after a verified read")
	}
	if got := server.requestCount() - before; got != requestsPerUnchangedCycle {
		t.Fatalf("unchanged cycle requests = %d, want %d", got, requestsPerUnchangedCycle)
	}
	if got := server.channelBodyCount(); got != 1 {
		t.Fatalf("channel document bodies = %d, want 1 after a 304", got)
	}
}

func TestGitHubSourceAdvancesWithTheChannelSequence(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	first := publishCatalog(t, server, "first-generation", testChannelSequence)
	attester := &recordingAttester{}
	source := newTestSource(t, server, WithAttester(attester.attest()))

	release, err := source.ReadChannel(t.Context())
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if release.Tag != first.Tag || release.Sequence != testChannelSequence {
		t.Fatalf("release = %s at sequence %d, want %s at %d",
			release.Tag, release.Sequence, first.Tag, testChannelSequence)
	}

	// The publisher promotes a later generation and the validator moves.
	next := publishCatalog(t, server, "next-generation", testChannelSequence+1)
	server.setChannelETag(testETag + "-moved")

	status, err := source.Changed(t.Context())
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}
	if !status.Changed {
		t.Fatal("Changed reported an unmoved channel after a promotion")
	}
	release, err = source.ReadChannel(t.Context())
	if err != nil {
		t.Fatalf("ReadChannel after promotion: %v", err)
	}
	if release.Tag != next.Tag || release.Sequence != testChannelSequence+1 {
		t.Fatalf("release = %s at sequence %d, want %s at %d",
			release.Tag, release.Sequence, next.Tag, testChannelSequence+1)
	}
	target, err := source.RollbackTarget()
	if err != nil {
		t.Fatalf("RollbackTarget: %v", err)
	}
	if target.Tag != next.Tag {
		t.Fatalf("rollback target = %q, want the promoted release %q", target.Tag, next.Tag)
	}
}

func TestGitHubSourceResetsTheStateWhenTheChannelChanges(t *testing.T) {
	t.Parallel()

	server := newFixtureServer(t)
	publishCatalog(t, server, "settled-generation", testChannelSequence)
	attester := &recordingAttester{}
	directory := t.TempDir()
	settled, err := New(
		WithAPIBaseURL(server.url()),
		WithRepository(testRepository),
		WithStateDirectory(directory),
		WithAttester(attester.attest()),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := settled.ReadChannel(t.Context()); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}

	// An operator points the source at a second branch that starts its own
	// sequence at one. A new channel is a new state, so the earlier floor and
	// the earlier validator never reject it.
	const renamed = artifact.ChannelName + "-canary"
	server.setChannelRef(renamed)
	restarted := publishCatalog(t, server, "restarted-generation", 1)
	moved, err := New(
		WithAPIBaseURL(server.url()),
		WithRepository(testRepository),
		WithStateDirectory(directory),
		WithChannel(renamed),
		WithAttester(attester.attest()),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	release, err := moved.ReadChannel(t.Context())
	if err != nil {
		t.Fatalf("ReadChannel on the renamed channel: %v", err)
	}
	if release.Tag != restarted.Tag || release.Sequence != 1 {
		t.Fatalf("release = %s at sequence %d, want %s at 1",
			release.Tag, release.Sequence, restarted.Tag)
	}
}
