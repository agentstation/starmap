package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/agentstation/starmap/internal/attestation"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
)

// Attester verifies one Sigstore bundle against a trust policy and one exact
// artifact digest.
//
// internal/attestation.Verify is the default. This function type exists
// because a caller may refresh the Sigstore engine on its own schedule. The
// default engine reaches no network.
type Attester func(
	ctx context.Context,
	bundleJSON []byte,
	artifactDigest string,
	policy attestation.Policy,
) (attestation.Result, error)

// digestHex returns the lowercase SHA-256 hexadecimal digest of data.
func digestHex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// verifyDigest reads the provenance the repository holds for one artifact
// digest and accepts the first bundle that satisfies the trust policy.
//
// GitHub may hold more than one attestation for a digest, so a single
// rejected bundle is not a trust decision. The last rejection is the reported
// failure when no bundle satisfies the policy.
func (c *cycle) verifyDigest(ctx context.Context, digest string) (attestation.Result, error) {
	found, err := c.bundles(ctx, digest)
	if err != nil {
		return attestation.Result{}, err
	}
	policy := c.client.config.policy()
	var lastErr error
	for _, bundleJSON := range found {
		result, err := c.client.config.Attester(ctx, bundleJSON, digest, policy)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return attestation.Result{}, lastErr
}

// verifyBytes verifies the provenance of exact bytes.
func (c *cycle) verifyBytes(ctx context.Context, data []byte) (attestation.Result, error) {
	return c.verifyDigest(ctx, digestHex(data))
}

// provenanceVerifier authenticates exact release archive bytes to the
// configured publisher identity. It implements artifact.PublisherVerifier, so
// artifact.VerifyRelease owns the checksum, statement, and compatibility
// checks and this type owns the publisher trust decision alone.
type provenanceVerifier struct {
	cycle  *cycle
	result attestation.Result
}

var _ artifact.PublisherVerifier = (*provenanceVerifier)(nil)

// VerifyPublisher implements artifact.PublisherVerifier.
func (v *provenanceVerifier) VerifyPublisher(ctx context.Context, name string, data []byte) error {
	if name != artifact.Filename {
		return sourceValidation("publisher.subject", name, "is not the catalog archive")
	}
	result, err := v.cycle.verifyBytes(ctx, data)
	if err != nil {
		return errors.WrapResource("verify", "catalog release provenance", name, err)
	}
	v.result = result
	return nil
}
