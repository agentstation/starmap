package starmap

import (
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// chainRejected is the safe reason code of a refused source chain.
const chainRejected = "chain_rejected"

// acceptSourceChain decides whether the runtime may activate one upstream
// read. A cascade of Starmap runtimes can form a loop that serves its own
// catalog back to itself. The loop then reports fresh forever while no origin
// catalog moves. URL comparison cannot detect the loop, because a load
// balancer, a DNS name, and a proxy all reach one node under different
// addresses. The chain therefore carries stable identities, and this check
// runs on those identities before the runtime retains the generation.
//
// The rules are three. A chain that names this instance is a self reference. A
// chain that names a declared alias of this instance is the same self
// reference under another name. A chain that repeats any identity closes a
// cycle between two or more nodes. The runtime refuses a chain longer than the
// hop budget too, because an unbounded cascade multiplies the origin latency.
func acceptSourceChain(
	instance string,
	aliases []string,
	maxHops int,
	chain []SourceHop,
) error {
	if maxHops > 0 && len(chain) > maxHops {
		return &errors.ValidationError{
			Field:   "catalog_source.chain.hops",
			Value:   len(chain),
			Message: "exceeds the configured maximum hop count",
		}
	}
	seen := make(map[string]struct{}, len(chain))
	for _, hop := range chain {
		identity := strings.TrimSpace(hop.Identity)
		if identity == "" {
			continue
		}
		if namesInstance(identity, instance, aliases) {
			return &errors.ConflictError{
				Resource: "catalog source chain",
				Expected: "a chain that excludes this runtime",
				Actual:   identity,
				Message:  "a Starmap runtime cannot read its own catalog",
			}
		}
		if _, repeated := seen[identity]; repeated {
			return &errors.ConflictError{
				Resource: "catalog source chain",
				Expected: "one entry for each source identity",
				Actual:   identity,
				Message:  "a repeated identity closes a cycle",
			}
		}
		seen[identity] = struct{}{}
	}
	return nil
}

// namesInstance reports whether one chain identity names this runtime.
func namesInstance(identity, instance string, aliases []string) bool {
	if instance != "" && identity == instance {
		return true
	}
	return slices.Contains(aliases, identity)
}
