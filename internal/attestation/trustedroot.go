package attestation

import (
	_ "embed"
)

// publicGoodTrustedRoot is the Sigstore public-good trusted root that ships
// with the binary. It is line 0 of `gh attestation trusted-root`, which is the
// same document the GitHub CLI verifies with.
//
// The compiled copy makes offline verification work with no network call. A
// connected caller refreshes the root through TUF and passes the fresh bytes
// through Policy.TrustedRootJSON instead.
//
//go:embed roots/sigstore-public-good-trusted-root.json
var publicGoodTrustedRoot []byte

// DefaultTrustedRootJSON returns a caller-owned copy of the compiled Sigstore
// public-good trusted root.
//
// The compiled root is a fixed snapshot. Sigstore rotates its keys, so a
// long-lived process should refresh the root through TUF and override it.
func DefaultTrustedRootJSON() []byte {
	return append([]byte(nil), publicGoodTrustedRoot...)
}
