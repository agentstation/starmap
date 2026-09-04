// Package source owns the upstream source contract of a connected catalog
// runtime. It holds the source interface, the two optional source
// capabilities, and the single upstream observation that a read returns.
//
// The package is a leaf. It implements no source and opens no connection. A
// package that implements a source therefore depends on this contract alone.
// It never depends on the attested source machinery that the runtime package
// carries.
package source

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/runtime/status"
)

// Source reads one upstream immutable catalog generation. Every built-in
// source verifies its evidence before it returns a generation.
type Source interface {
	// Identity returns the safe identity of the source. It names no URL, no
	// host, and no credential.
	Identity() string

	// Read returns the current upstream generation. A source that finds no
	// change reports Changed false and carries no generation.
	Read(ctx context.Context) (Read, error)
}

// Watcher is an optional Source that reports an upstream change as it
// arrives. A reactive source, such as one Starmap cascaded onto another,
// learns of a publication on its own stream. The runtime then refreshes on
// that wake and waits for no poll boundary. A delta crosses a cascade in
// seconds instead of in one poll interval.
//
// The channel carries an empty value for each change and holds at most one
// pending wake. A closed channel means the source reports no further change,
// and the runtime falls back to its poll interval.
type Watcher interface {
	Source

	// Changes reports each upstream change as one wake.
	Changes() <-chan struct{}
}

// IdentityAdopter is an optional Source that takes the fleet instance
// identity of its runtime. The source and the runtime then spread their work
// on one identity, so a replica keeps one stable phase for every controller it
// owns. Open hands the identity over before the first read.
type IdentityAdopter interface {
	Source

	// AdoptInstanceIdentity takes the derived instance identity of the runtime.
	AdoptInstanceIdentity(instance string)
}

// Read is one upstream observation.
type Read struct {
	// Changed reports whether the upstream generation moved.
	Changed bool

	// Generation is the verified immutable catalog generation. It is empty
	// when Changed is false.
	Generation catalogs.Generation

	// PublishedAt is the upstream publication time.
	PublishedAt time.Time

	// ChannelUpdatedAt is when the upstream channel last moved.
	ChannelUpdatedAt time.Time

	// Chain is the sanitized upstream source chain, nearest hop first.
	Chain []status.SourceHop

	// Health is the upstream-reported health of the source itself.
	Health status.Health
}
