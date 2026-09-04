package attestation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/errors"
)

// The fixtures come from one public Starmap catalog release. The command
// "gh attestation download starmap-catalog.tar.gz --repo agentstation/starmap"
// captured the attestation lines, and "gh attestation trusted-root" captured
// the trusted root. Every test reads these files and reaches no network.
const (
	// releaseTag is the captured public prerelease.
	releaseTag = "catalog-semantic-f03df976d3164471b47fe874e23b4b45a13e2dc4d7dc2e83edfe55b43a353dc4"

	// artifactDigest is the SHA-256 digest of starmap-catalog.tar.gz in that
	// release.
	artifactDigest = "92f1fb8bc52ed57eceda71cc101c43f6091bdce9e992345d220a2b1fd69b8adc"

	// signerIdentity is the certificate subject alternative name that the
	// catalog-generation workflow received.
	signerIdentity = "https://github.com/agentstation/starmap/.github/workflows/catalog-generation.yaml@refs/heads/main"

	// sourceRepositoryURI is the verified source repository.
	sourceRepositoryURI = "https://github.com/agentstation/starmap"

	// sourceRepositoryDigest is the commit that produced the artifact.
	sourceRepositoryDigest = "ab84e57d5fceb907c57994db9a8a6a860d58a6d3"

	bundleFixture      = "catalog-provenance-bundle.json"
	trustedRootFixture = "sigstore-public-good-trusted-root.json"
	captureFixture     = "gh-attestation-download.jsonl"
)

// readFixture reads one testdata file.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// starmapPolicy is the production catalog publisher policy.
func starmapPolicy(t *testing.T) attestation.Policy {
	t.Helper()
	return attestation.Policy{
		Repository:            "agentstation/starmap",
		Workflow:              ".github/workflows/catalog-generation.yaml",
		Issuer:                attestation.GitHubOIDCIssuer,
		PredicateType:         attestation.BuildProvenancePredicateType,
		TrustedRootJSON:       readFixture(t, trustedRootFixture),
		DenySelfHostedRunners: true,
	}
}

func TestVerifyAcceptsPublishedCatalogProvenance(t *testing.T) {
	t.Parallel()

	result, err := attestation.Verify(t.Context(), readFixture(t, bundleFixture), artifactDigest, starmapPolicy(t))
	if err != nil {
		t.Fatalf("Verify %s: %v", releaseTag, err)
	}
	for _, field := range []struct {
		name string
		got  string
		want string
	}{
		{"PredicateType", result.PredicateType, attestation.BuildProvenancePredicateType},
		{"SignerIdentity", result.SignerIdentity, signerIdentity},
		{"SourceRepositoryURI", result.SourceRepositoryURI, sourceRepositoryURI},
		{"SourceRepositoryDigest", result.SourceRepositoryDigest, sourceRepositoryDigest},
		{"RunnerEnvironment", result.RunnerEnvironment, attestation.HostedRunnerEnvironment},
	} {
		if field.got != field.want {
			t.Errorf("%s = %q, want %q", field.name, field.got, field.want)
		}
	}
	if result.ObservedAt.IsZero() {
		t.Error("ObservedAt is zero")
	}
}

func TestVerifyRejectsUntrustedEvidence(t *testing.T) {
	t.Parallel()

	valid := readFixture(t, bundleFixture)
	tests := []struct {
		name    string
		bundle  []byte
		digest  string
		mutate  func(*attestation.Policy)
		wantErr any
	}{
		{
			name:   "wrong repository",
			bundle: valid,
			digest: artifactDigest,
			mutate: func(p *attestation.Policy) { p.Repository = "agentstation/starport" },
			// The certificate identity no longer matches the signer workflow.
			wantErr: &attestation.TrustError{},
		},
		{
			name:    "wrong workflow",
			bundle:  valid,
			digest:  artifactDigest,
			mutate:  func(p *attestation.Policy) { p.Workflow = ".github/workflows/release.yaml" },
			wantErr: &attestation.TrustError{},
		},
		{
			name:    "wrong issuer",
			bundle:  valid,
			digest:  artifactDigest,
			mutate:  func(p *attestation.Policy) { p.Issuer = "https://accounts.google.com" },
			wantErr: &attestation.TrustError{},
		},
		{
			name:    "wrong predicate type",
			bundle:  valid,
			digest:  artifactDigest,
			mutate:  func(p *attestation.Policy) { p.PredicateType = "https://slsa.dev/provenance/v0.2" },
			wantErr: &attestation.TrustError{},
		},
		{
			name:    "wrong digest",
			bundle:  valid,
			digest:  strings.Repeat("a", 64),
			wantErr: &attestation.TrustError{},
		},
		{
			name:    "tampered bundle",
			bundle:  tamperedBundle(t, valid),
			digest:  artifactDigest,
			wantErr: &attestation.TrustError{},
		},
		{
			name:    "undecodable bundle",
			bundle:  []byte("{"),
			digest:  artifactDigest,
			wantErr: &errors.ParseError{},
		},
		{
			name:    "undecodable trusted root",
			bundle:  valid,
			digest:  artifactDigest,
			mutate:  func(p *attestation.Policy) { p.TrustedRootJSON = []byte("{") },
			wantErr: &errors.ParseError{},
		},
		{
			name:    "empty bundle",
			bundle:  nil,
			digest:  artifactDigest,
			wantErr: &errors.ValidationError{},
		},
		{
			name:    "malformed digest",
			bundle:  valid,
			digest:  "sha256:" + artifactDigest,
			wantErr: &errors.ValidationError{},
		},
		{
			name:    "missing trusted root",
			bundle:  valid,
			digest:  artifactDigest,
			mutate:  func(p *attestation.Policy) { p.TrustedRootJSON = nil },
			wantErr: &errors.ValidationError{},
		},
		{
			name:    "missing repository",
			bundle:  valid,
			digest:  artifactDigest,
			mutate:  func(p *attestation.Policy) { p.Repository = "" },
			wantErr: &errors.ValidationError{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			policy := starmapPolicy(t)
			if test.mutate != nil {
				test.mutate(&policy)
			}
			result, err := attestation.Verify(t.Context(), test.bundle, test.digest, policy)
			if err == nil {
				t.Fatalf("Verify accepted %s: %+v", test.name, result)
			}
			assertErrorType(t, err, test.wantErr)
		})
	}
}

func TestVerifyRejectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := attestation.Verify(ctx, readFixture(t, bundleFixture), artifactDigest, starmapPolicy(t)); err == nil {
		t.Fatal("Verify accepted a canceled context")
	}
}

// TestBundleFixtureMatchesGitHubCLICapture proves that the tests verify one
// unchanged line of the raw gh attestation download output.
func TestBundleFixtureMatchesGitHubCLICapture(t *testing.T) {
	t.Parallel()

	want := bytes.TrimSpace(readFixture(t, bundleFixture))
	for line := range bytes.SplitSeq(readFixture(t, captureFixture), []byte("\n")) {
		if bytes.Equal(bytes.TrimSpace(line), want) {
			return
		}
	}
	t.Fatalf("%s is not a document of %s", bundleFixture, captureFixture)
}

// tamperedBundle re-encodes the bundle with one changed statement byte and the
// original signature.
func tamperedBundle(t *testing.T, valid []byte) []byte {
	t.Helper()
	var document map[string]json.RawMessage
	if err := json.Unmarshal(valid, &document); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(document["dsseEnvelope"], &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var payload string
	if err := json.Unmarshal(envelope["payload"], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	statement, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	tampered := bytes.Replace(statement, []byte(attestation.HostedRunnerEnvironment), []byte("self-hosted--"), 1)
	if bytes.Equal(tampered, statement) {
		t.Fatal("the statement carries no runner environment to change")
	}
	envelope["payload"], err = json.Marshal(base64.StdEncoding.EncodeToString(tampered))
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	document["dsseEnvelope"], err = json.Marshal(envelope)
	if err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode bundle: %v", err)
	}
	return encoded
}

// assertErrorType reports whether err has the same concrete type as want.
func assertErrorType(t *testing.T, err error, want any) {
	t.Helper()
	switch want.(type) {
	case *attestation.TrustError:
		var target *attestation.TrustError
		if !stderrors.As(err, &target) {
			t.Fatalf("error %v is not a *attestation.TrustError", err)
		}
	case *errors.ParseError:
		var target *errors.ParseError
		if !stderrors.As(err, &target) {
			t.Fatalf("error %v is not a *errors.ParseError", err)
		}
	case *errors.ValidationError:
		var target *errors.ValidationError
		if !stderrors.As(err, &target) {
			t.Fatalf("error %v is not a *errors.ValidationError", err)
		}
	default:
		t.Fatalf("unsupported expected error type %T", want)
	}
}
