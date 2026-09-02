// Package attestation verifies GitHub build-provenance attestations for
// Starmap catalog release artifacts.
//
// The package separates two responsibilities. The Sigstore engine
// (github.com/sigstore/sigstore-go) validates the bundle, the certificate
// chain, the transparency evidence, the observer timestamps, and the artifact
// digest. This package owns the Starmap trust policy. The policy binds the
// verified certificate and statement to one repository, one workflow, one
// OIDC issuer, one predicate type, and one artifact digest.
//
// Verify does not use the network. The caller supplies the bundle bytes, the
// artifact digest, and the trusted-root document. A connected caller
// refreshes the trusted root through TUF and passes the result here.
package attestation

import (
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// GitHubOIDCIssuer is the OIDC issuer of every GitHub Actions workload
	// identity.
	GitHubOIDCIssuer = "https://token.actions.githubusercontent.com"

	// BuildProvenancePredicateType is the in-toto predicate type that
	// actions/attest-build-provenance writes.
	BuildProvenancePredicateType = "https://slsa.dev/provenance/v1"

	// HostedRunnerEnvironment is the value that a GitHub-hosted runner
	// reports. A self-hosted runner reports a different value.
	HostedRunnerEnvironment = "github-hosted"

	// DigestAlgorithm is the only artifact digest algorithm the policy accepts.
	DigestAlgorithm = "sha256"

	// gitHubURL prefixes every GitHub Actions signer identity.
	gitHubURL = "https://github.com"

	// digestHexLength is the hexadecimal length of a SHA-256 digest.
	digestHexLength = 64

	// maxBundleBytes bounds one attestation bundle document. A GitHub build
	// provenance bundle is about 11 KiB, so this leaves wide headroom.
	maxBundleBytes = 1 << 20

	// maxTrustedRootBytes bounds one trusted-root document. The Sigstore
	// public-good root is about 6 KiB and the GitHub root is about 29 KiB.
	maxTrustedRootBytes = 4 << 20

	// evidenceThreshold is the number of independent signed certificate
	// timestamps, transparency log entries, and observer timestamps the policy
	// requires.
	evidenceThreshold = 1
)

// Policy binds a verified attestation to one Starmap publisher identity.
// Verify requires every field.
type Policy struct {
	// Repository is the owner and name of the signing repository, such as
	// "agentstation/starmap".
	Repository string

	// Workflow is the repository-relative path of the signing workflow, such
	// as ".github/workflows/catalog-generation.yaml".
	Workflow string

	// Issuer is the expected OIDC issuer. Use GitHubOIDCIssuer.
	Issuer string

	// PredicateType is the expected in-toto predicate type. Use
	// BuildProvenancePredicateType.
	PredicateType string

	// TrustedRootJSON is a Sigstore trusted-root document. The caller reads
	// and refreshes it.
	TrustedRootJSON []byte

	// DenySelfHostedRunners rejects an attestation that a self-hosted runner
	// produced.
	DenySelfHostedRunners bool
}

// Result reports the publisher facts that verification proved.
type Result struct {
	// PredicateType is the verified in-toto predicate type.
	PredicateType string

	// SignerIdentity is the verified certificate subject alternative name.
	SignerIdentity string

	// SourceRepositoryURI is the verified source repository.
	SourceRepositoryURI string

	// SourceRepositoryDigest is the verified commit that built the artifact.
	SourceRepositoryDigest string

	// RunnerEnvironment is the verified runner environment.
	RunnerEnvironment string

	// ObservedAt is the earliest verified observer timestamp.
	ObservedAt time.Time
}

// Verify checks one Sigstore bundle against the policy and the artifact
// digest. The digest is a lowercase hexadecimal SHA-256 digest without an
// algorithm prefix.
//
// Verify returns a *errors.ValidationError for an unusable argument, a
// *errors.ParseError for an undecodable document, and a *TrustError when the
// evidence does not satisfy the policy.
func Verify(ctx context.Context, bundleJSON []byte, artifactDigest string, policy Policy) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := policy.validate(); err != nil {
		return Result{}, err
	}
	digest, err := decodeDigest(artifactDigest)
	if err != nil {
		return Result{}, err
	}
	if len(bundleJSON) == 0 || len(bundleJSON) > maxBundleBytes {
		return Result{}, errors.NewValidationError("bundleJSON", len(bundleJSON),
			fmt.Sprintf("bundle must hold 1 to %d bytes", maxBundleBytes))
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(policy.TrustedRootJSON)
	if err != nil {
		return Result{}, errors.NewParseError("json", "trusted root", "cannot decode trusted root", err)
	}

	signed := &bundle.Bundle{}
	if err := signed.UnmarshalJSON(bundleJSON); err != nil {
		return Result{}, errors.NewParseError("json", "attestation bundle", "cannot decode bundle", err)
	}

	verifier, err := verify.NewVerifier(trustedRoot,
		verify.WithSignedCertificateTimestamps(evidenceThreshold),
		verify.WithTransparencyLog(evidenceThreshold),
		verify.WithObserverTimestamps(evidenceThreshold),
	)
	if err != nil {
		return Result{}, &TrustError{Stage: "verifier", Message: "cannot build the Sigstore verifier", Err: err}
	}

	identity, err := policy.certificateIdentity()
	if err != nil {
		return Result{}, err
	}

	result, err := verifier.Verify(signed, verify.NewPolicy(
		verify.WithArtifactDigest(DigestAlgorithm, digest),
		verify.WithCertificateIdentity(identity),
	))
	if err != nil {
		return Result{}, &TrustError{Stage: "signature", Message: "the bundle does not satisfy the Starmap policy", Err: err}
	}
	return policy.result(result)
}

// TrustError reports evidence that does not satisfy the Starmap policy.
type TrustError struct {
	// Stage names the policy check that failed.
	Stage string

	// Expected is the value the policy requires, when the stage compares one.
	Expected string

	// Actual is the value the attestation carried, when the stage compares one.
	Actual string

	// Message describes the failure.
	Message string

	// Err is the underlying engine error, when one exists.
	Err error
}

// Error implements the error interface.
func (e *TrustError) Error() string {
	if e.Expected != "" || e.Actual != "" {
		return fmt.Sprintf("attestation trust failure at %s: %s (expected %q, got %q)",
			e.Stage, e.Message, e.Expected, e.Actual)
	}
	return fmt.Sprintf("attestation trust failure at %s: %s", e.Stage, e.Message)
}

// Unwrap implements errors.Unwrap.
func (e *TrustError) Unwrap() error {
	return e.Err
}

// validate reports whether every policy field is usable.
func (p Policy) validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"Repository", p.Repository},
		{"Workflow", p.Workflow},
		{"Issuer", p.Issuer},
		{"PredicateType", p.PredicateType},
	} {
		if field.value == "" {
			return errors.NewValidationError("Policy."+field.name, field.value, "value is required")
		}
	}
	if strings.Count(p.Repository, "/") != 1 {
		return errors.NewValidationError("Policy.Repository", p.Repository, "value must be owner/name")
	}
	if strings.HasPrefix(p.Workflow, "/") {
		return errors.NewValidationError("Policy.Workflow", p.Workflow, "value must be a repository-relative path")
	}
	if len(p.TrustedRootJSON) == 0 || len(p.TrustedRootJSON) > maxTrustedRootBytes {
		return errors.NewValidationError("Policy.TrustedRootJSON", len(p.TrustedRootJSON),
			fmt.Sprintf("trusted root must hold 1 to %d bytes", maxTrustedRootBytes))
	}
	return nil
}

// signerURL is the certificate identity prefix of the policy workflow.
func (p Policy) signerURL() string {
	return gitHubURL + "/" + p.Repository + "/" + p.Workflow
}

// repositoryURL is the expected source repository of the policy.
func (p Policy) repositoryURL() string {
	return gitHubURL + "/" + p.Repository
}

// certificateIdentity binds the issuer and the exact signer workflow. The
// pattern accepts any Git ref, because the publisher can run from a renamed
// default branch.
func (p Policy) certificateIdentity() (verify.CertificateIdentity, error) {
	pattern := "^" + regexp.QuoteMeta(p.signerURL()+"@refs/") + `[\x21-\x7e]+$`
	identity, err := verify.NewShortCertificateIdentity(p.Issuer, "", "", pattern)
	if err != nil {
		return verify.CertificateIdentity{}, errors.NewValidationError("Policy.Workflow", p.Workflow,
			"cannot build a certificate identity from the policy")
	}
	return identity, nil
}

// result checks the statement and certificate claims that the engine does not
// bind, and then reports the proved publisher facts.
func (p Policy) result(verified *verify.VerificationResult) (Result, error) {
	if verified.Statement == nil {
		return Result{}, &TrustError{Stage: "statement", Message: "the bundle carries no in-toto statement"}
	}
	if verified.Statement.PredicateType != p.PredicateType {
		return Result{}, &TrustError{
			Stage:    "predicate",
			Expected: p.PredicateType,
			Actual:   verified.Statement.PredicateType,
			Message:  "unexpected predicate type",
		}
	}
	if verified.Signature == nil || verified.Signature.Certificate == nil {
		return Result{}, &TrustError{Stage: "certificate", Message: "the result carries no certificate summary"}
	}
	summary := verified.Signature.Certificate
	if summary.SourceRepositoryURI != p.repositoryURL() {
		return Result{}, &TrustError{
			Stage:    "repository",
			Expected: p.repositoryURL(),
			Actual:   summary.SourceRepositoryURI,
			Message:  "unexpected source repository",
		}
	}
	if p.DenySelfHostedRunners && summary.RunnerEnvironment != HostedRunnerEnvironment {
		return Result{}, &TrustError{
			Stage:    "runner",
			Expected: HostedRunnerEnvironment,
			Actual:   summary.RunnerEnvironment,
			Message:  "the policy denies a self-hosted runner",
		}
	}
	if len(verified.VerifiedTimestamps) == 0 {
		return Result{}, &TrustError{Stage: "timestamp", Message: "the result carries no verified timestamp"}
	}
	return Result{
		PredicateType:          verified.Statement.PredicateType,
		SignerIdentity:         summary.SubjectAlternativeName,
		SourceRepositoryURI:    summary.SourceRepositoryURI,
		SourceRepositoryDigest: summary.SourceRepositoryDigest,
		RunnerEnvironment:      summary.RunnerEnvironment,
		ObservedAt:             earliest(verified.VerifiedTimestamps),
	}, nil
}

// decodeDigest converts a lowercase hexadecimal SHA-256 digest into bytes.
func decodeDigest(artifactDigest string) ([]byte, error) {
	if len(artifactDigest) != digestHexLength {
		return nil, errors.NewValidationError("artifactDigest", artifactDigest,
			fmt.Sprintf("digest must hold %d hexadecimal characters", digestHexLength))
	}
	digest, err := hex.DecodeString(artifactDigest)
	if err != nil {
		return nil, errors.NewValidationError("artifactDigest", artifactDigest, "digest is not hexadecimal")
	}
	return digest, nil
}

// earliest reports the earliest verified observer timestamp.
func earliest(timestamps []verify.TimestampVerificationResult) time.Time {
	observed := timestamps[0].Timestamp
	for _, timestamp := range timestamps[1:] {
		if timestamp.Timestamp.Before(observed) {
			observed = timestamp.Timestamp
		}
	}
	return observed
}
