package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

const (
	// testRepository is the repository every fixture publishes from.
	testRepository = "agentstation/starmap"

	// manifestFixture is the committed generation manifest fixture.
	manifestFixture = "../../../pkg/catalogs/testdata/generation/manifest.json"

	// attestationFixtures holds the committed Sigstore evidence that CAT2.1
	// captured from one public Starmap catalog release.
	attestationFixtures = "../../attestation/testdata"

	// committedBundle is the real GitHub build-provenance bundle.
	committedBundle = "catalog-provenance-bundle.json"

	// committedArtifactDigest is the archive digest that bundle attests.
	committedArtifactDigest = "92f1fb8bc52ed57eceda71cc101c43f6091bdce9e992345d220a2b1fd69b8adc"

	// stubBundle is the placeholder evidence that a stub attester accepts.
	stubBundle = `{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`

	// testChannelSequence is the first published channel sequence.
	testChannelSequence = 7

	// testETag is the channel release validator.
	testETag = `W/"catalog-latest-1"`
)

// readFixture reads one committed evidence file.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(attestationFixtures, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// attesterCall records one verification request.
type attesterCall struct {
	Digest string
	Bundle []byte
	Policy attestation.Policy
}

// recordingAttester stands in for the Sigstore engine. It records every
// request and answers from a digest allowlist, so a test proves the glue
// without a signing key.
type recordingAttester struct {
	mu     sync.Mutex
	calls  []attesterCall
	reject map[string]error
}

// attest returns the Attester of this recorder.
func (a *recordingAttester) attest() Attester {
	return func(
		_ context.Context,
		bundleJSON []byte,
		artifactDigest string,
		policy attestation.Policy,
	) (attestation.Result, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.calls = append(a.calls, attesterCall{
			Digest: artifactDigest,
			Bundle: append([]byte(nil), bundleJSON...),
			Policy: policy,
		})
		if err, found := a.reject[artifactDigest]; found {
			return attestation.Result{}, err
		}
		return attestation.Result{
			PredicateType:     policy.PredicateType,
			SignerIdentity:    "https://github.com/" + policy.Repository + "/" + policy.Workflow,
			RunnerEnvironment: attestation.HostedRunnerEnvironment,
			ObservedAt:        time.Unix(1, 0).UTC(),
		}, nil
	}
}

// digests returns every digest that the recorder verified.
func (a *recordingAttester) digests() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	seen := make([]string, 0, len(a.calls))
	for _, call := range a.calls {
		seen = append(seen, call.Digest)
	}
	return seen
}

// releaseFixture is one published release on the fixture server.
type releaseFixture struct {
	Tag    string
	Assets []assetFixture
}

// assetFixture is one published asset.
type assetFixture struct {
	Name string
	Body []byte
}

// fixtureServer serves releases, assets, and attestations from memory. It
// reaches no network and holds no credential.
type fixtureServer struct {
	t      *testing.T
	server *httptest.Server

	mu          sync.Mutex
	releases    map[string]releaseFixture
	bundles     map[string][]byte
	rateHeaders map[string]string
	channelETag string
	requests    int
}

// newFixtureServer starts one hermetic GitHub API stand-in.
func newFixtureServer(t *testing.T) *fixtureServer {
	t.Helper()
	fixture := &fixtureServer{
		t:           t,
		releases:    make(map[string]releaseFixture),
		bundles:     make(map[string][]byte),
		rateHeaders: make(map[string]string),
		channelETag: testETag,
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.serve))
	t.Cleanup(fixture.server.Close)
	return fixture
}

// url returns the API root of the fixture server.
func (f *fixtureServer) url() string { return f.server.URL }

// requestCount returns the number of requests the server answered.
func (f *fixtureServer) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

// setRateHeaders replaces the rate-limit headers on every reply.
func (f *fixtureServer) setRateHeaders(headers map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rateHeaders = headers
}

// publish adds one release and the provenance of each of its assets.
func (f *fixtureServer) publish(release releaseFixture) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases[release.Tag] = release
	for _, asset := range release.Assets {
		f.bundles[hexDigest(asset.Body)] = []byte(stubBundle)
	}
}

// setBundle replaces the evidence bound to one digest.
func (f *fixtureServer) setBundle(digest string, bundleJSON []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[digest] = bundleJSON
}

// setChannelETag replaces the channel release validator.
func (f *fixtureServer) setChannelETag(etag string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channelETag = etag
}

func (f *fixtureServer) serve(writer http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	f.requests++
	for name, value := range f.rateHeaders {
		writer.Header().Set(name, value)
	}
	f.mu.Unlock()

	path := request.URL.Path
	prefix := "/repos/" + testRepository + "/"
	switch {
	case strings.HasPrefix(path, prefix+"releases/tags/"):
		f.serveRelease(writer, request, strings.TrimPrefix(path, prefix+"releases/tags/"))
	case strings.HasPrefix(path, prefix+"attestations/"):
		f.serveAttestation(writer, strings.TrimPrefix(path, prefix+"attestations/sha256:"))
	case strings.HasPrefix(path, "/assets/"):
		f.serveAsset(writer, strings.TrimPrefix(path, "/assets/"))
	default:
		writer.WriteHeader(http.StatusNotFound)
	}
}

func (f *fixtureServer) serveRelease(writer http.ResponseWriter, request *http.Request, tag string) {
	f.mu.Lock()
	release, found := f.releases[tag]
	etag := f.channelETag
	f.mu.Unlock()
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if tag == artifact.ChannelName {
		writer.Header().Set("ETag", etag)
		if request.Header.Get("If-None-Match") == etag {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
	}
	assets := make([]releaseAsset, 0, len(release.Assets))
	for _, asset := range release.Assets {
		assets = append(assets, releaseAsset{
			Name: asset.Name,
			Size: int64(len(asset.Body)),
			URL:  f.server.URL + "/assets/" + tag + "/" + asset.Name,
		})
	}
	writer.Header().Set("Content-Type", "application/json")
	f.writeJSON(writer, releaseDocument{TagName: tag, Assets: assets})
}

func (f *fixtureServer) serveAsset(writer http.ResponseWriter, reference string) {
	tag, name, found := strings.Cut(reference, "/")
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	f.mu.Lock()
	release := f.releases[tag]
	f.mu.Unlock()
	for _, asset := range release.Assets {
		if asset.Name == name {
			writer.Header().Set("Content-Type", "application/octet-stream")
			if _, err := writer.Write(asset.Body); err != nil {
				f.t.Errorf("write asset %s: %v", name, err)
			}
			return
		}
	}
	writer.WriteHeader(http.StatusNotFound)
}

func (f *fixtureServer) serveAttestation(writer http.ResponseWriter, digest string) {
	f.mu.Lock()
	bundleJSON, found := f.bundles[digest]
	f.mu.Unlock()
	if !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	f.writeJSON(writer, attestationDocument{
		Attestations: []attestationEntry{{Bundle: bundleJSON}},
	})
}

func (f *fixtureServer) writeJSON(writer http.ResponseWriter, document any) {
	if err := json.NewEncoder(writer).Encode(document); err != nil {
		f.t.Errorf("encode fixture reply: %v", err)
	}
}

// hexDigest returns the lowercase SHA-256 digest of data.
func hexDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// testGeneration builds one deterministic catalog generation.
func testGeneration(t *testing.T, id string) catalogs.Generation {
	t.Helper()
	manifestData, err := os.ReadFile(manifestFixture)
	if err != nil {
		t.Fatalf("read manifest fixture: %v", err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		t.Fatalf("parse manifest fixture: %v", err)
	}
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("build empty catalog: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("encode catalog payload: %v", err)
	}
	manifest.GenerationID = id
	manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		t.Fatalf("validate generation: %v", err)
	}
	return generation
}

// releaseAssetsOf packages one generation as the three published assets.
func releaseAssetsOf(t *testing.T, generation catalogs.Generation) []assetFixture {
	t.Helper()
	bundle, err := artifact.Build(generation)
	if err != nil {
		t.Fatalf("build artifact: %v", err)
	}
	checksum := strings.TrimPrefix(bundle.Checksum, artifact.ChecksumPrefix) +
		"  " + artifact.Filename + "\n"
	return []assetFixture{
		{Name: artifact.Filename, Body: bundle.Data},
		{Name: artifact.ChecksumFilename, Body: []byte(checksum)},
		{Name: artifact.AttestationFilename, Body: bundle.Attestation},
	}
}

// channelAssetsOf records every asset in the channel document form.
func channelAssetsOf(assets []assetFixture) []artifact.ChannelAsset {
	recorded := make([]artifact.ChannelAsset, 0, len(assets))
	for _, asset := range assets {
		recorded = append(recorded, artifact.ChannelAsset{
			Name:      asset.Name,
			MediaType: artifact.MediaType,
			Checksum:  artifact.ChecksumPrefix + hexDigest(asset.Body),
			SizeBytes: int64(len(asset.Body)),
		})
	}
	return recorded
}

// publication is one complete fixture publication.
type publication struct {
	Generation catalogs.Generation
	Tag        string
	Assets     []assetFixture
	Channel    artifact.Channel
	ChannelDoc []byte
}

// publishCatalog stages one immutable release and the channel that selects
// it. A zero sequence publishes no channel document.
func publishCatalog(t *testing.T, server *fixtureServer, id string, sequence uint64) publication {
	t.Helper()
	generation := testGeneration(t, id)
	assets := releaseAssetsOf(t, generation)
	digest := strings.TrimPrefix(generation.Manifest.Payload.Checksum, artifact.ChecksumPrefix)
	tag, err := artifact.ReleaseTag(digest)
	if err != nil {
		t.Fatalf("release tag: %v", err)
	}
	server.publish(releaseFixture{Tag: tag, Assets: assets})
	result := publication{Generation: generation, Tag: tag, Assets: assets}
	if sequence == 0 {
		return result
	}
	document := artifact.Channel{
		SchemaVersion:    artifact.ChannelSchemaVersion,
		Name:             artifact.ChannelName,
		Sequence:         sequence,
		ChannelUpdatedAt: time.Unix(int64(sequence), 0).UTC(),
		GenerationID:     id,
		Tag:              tag,
		CatalogDigest:    digest,
		PublishedAt:      time.Unix(int64(sequence), 0).UTC(),
		Assets:           channelAssetsOf(assets),
	}
	encoded, err := artifact.EncodeChannel(document)
	if err != nil {
		t.Fatalf("encode channel: %v", err)
	}
	server.publish(releaseFixture{
		Tag:    artifact.ChannelName,
		Assets: []assetFixture{{Name: artifact.ChannelFilename, Body: encoded}},
	})
	result.Channel = document
	result.ChannelDoc = encoded
	return result
}

// newTestSource builds a source that reads one fixture server.
func newTestSource(t *testing.T, server *fixtureServer, opts ...Option) *Source {
	t.Helper()
	base := []Option{
		WithAPIBaseURL(server.url()),
		WithStateDirectory(t.TempDir()),
		WithRepository(testRepository),
	}
	source, err := New(append(base, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return source
}

// legacyTag returns a retired-namespace tag for one digest.
func legacyTag(prefix, digest string) string {
	return fmt.Sprintf("%s%s", prefix, digest)
}
