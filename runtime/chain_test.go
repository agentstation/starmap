package runtime

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// TestRuntimeRefusesSelfAliasAndCyclicSourceChains proves the three chain rules
// of a cascade. The runtime refuses a chain that names this instance. It also
// refuses a chain that names a declared alias of it, and a chain that repeats
// one identity. The runtime keeps the embedded catalog, so it never serves a
// catalog that a loop fed back to it.
func TestRuntimeRefusesSelfAliasAndCyclicSourceChains(t *testing.T) {
	t.Parallel()
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	payload := testCatalogPayload(t, "cascade-provider", "cascade-model", "Cascade Model")

	for _, test := range []struct {
		name    string
		aliases []string
		chain   []SourceHop
		refused string
	}{
		{
			name:    "self reference",
			chain:   []SourceHop{{Identity: "instance-under-test"}},
			refused: "instance-under-test",
		},
		{
			name:    "alias of this instance",
			aliases: []string{"other-name", "load-balancer"},
			chain: []SourceHop{
				{Identity: "upstream"},
				{Identity: "load-balancer"},
			},
			refused: "load-balancer",
		},
		{
			name: "two node cycle",
			chain: []SourceHop{
				{Identity: "upstream"},
				{Identity: "downstream"},
				{Identity: "upstream"},
			},
			refused: "upstream",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := newStubSource("cascade-source")
			read := testSourceRead(t, "generation-cascade", payload, published)
			read.Chain = test.chain
			source.replies = []SourceRead{read}
			runtime := openTestRuntime(t,
				WithSource(source),
				WithSchedulerIdentity("instance-under-test"),
				WithSourceAliases(test.aliases...),
			)

			_, err := runtime.RefreshSource(context.Background())
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("RefreshSource error = %v, want a ConflictError", err)
			}
			if conflict.Actual != test.refused {
				t.Fatalf("refused identity = %q, want %q", conflict.Actual, test.refused)
			}
			status := runtime.Status()
			if status.SourceHealth != HealthUnavailable {
				t.Fatalf("source health = %q, want %q", status.SourceHealth, HealthUnavailable)
			}
			if status.SourceReason != chainRejected {
				t.Fatalf("source reason = %q, want %q", status.SourceReason, chainRejected)
			}
			if !status.Fallback {
				t.Fatal("the runtime kept a generation that a loop returned to it")
			}
		})
	}
}

// TestRuntimeRefusesAnOverlongSourceChain proves the hop bound. An unbounded
// cascade multiplies the origin latency, so the runtime refuses the read before
// it keeps the generation.
func TestRuntimeRefusesAnOverlongSourceChain(t *testing.T) {
	t.Parallel()
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	payload := testCatalogPayload(t, "cascade-provider", "cascade-model", "Cascade Model")
	read := testSourceRead(t, "generation-long", payload, published)
	read.Chain = []SourceHop{
		{Identity: "hop-a"}, {Identity: "hop-b"}, {Identity: "hop-c"},
	}
	source := newStubSource("cascade-source")
	source.replies = []SourceRead{read}
	runtime := openTestRuntime(t,
		WithSource(source),
		WithSchedulerIdentity("instance-under-test"),
		WithSourceMaxHops(2),
	)

	_, err := runtime.RefreshSource(context.Background())
	var invalid *errors.ValidationError
	if !stderrors.As(err, &invalid) {
		t.Fatalf("RefreshSource error = %v, want a ValidationError", err)
	}
	if status := runtime.Status(); status.SourceReason != chainRejected {
		t.Fatalf("source reason = %q, want %q", status.SourceReason, chainRejected)
	}
}

// TestRuntimeGradesThePropagatedChannelTime proves that a hop grades the origin
// publication time and not only its own check. The served generation is new
// here, so a runtime that graded its own check would report a current channel.
func TestRuntimeGradesThePropagatedChannelTime(t *testing.T) {
	t.Parallel()
	clock := &testClock{now: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	payload := testCatalogPayload(t, "cascade-provider", "cascade-model", "Cascade Model")
	origin := clock.now.Add(-12 * time.Hour)
	read := testSourceRead(t, "generation-propagated", payload, clock.now)
	read.ChannelUpdatedAt = origin
	read.Chain = []SourceHop{{Identity: "upstream", Health: HealthOK}}
	source := newStubSource("cascade-source")
	source.replies = []SourceRead{read}
	runtime := openTestRuntime(t, WithSource(source), WithClock(clock.Now))

	if _, err := runtime.RefreshSource(context.Background()); err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	status := runtime.Status()
	if !status.ChannelUpdatedAt.Equal(origin) {
		t.Fatalf("channel time = %s, want the propagated origin time %s",
			status.ChannelUpdatedAt, origin)
	}
	if status.ChannelFreshness != FreshnessCritical {
		t.Fatalf("channel freshness = %q, want %q",
			status.ChannelFreshness, FreshnessCritical)
	}
	if status.Freshness != FreshnessCurrent {
		t.Fatalf("catalog freshness = %q, want %q, because the hop published now",
			status.Freshness, FreshnessCurrent)
	}
	if len(status.Chain) != 1 || status.Chain[0].Identity != "upstream" {
		t.Fatalf("chain = %#v, want the one upstream hop", status.Chain)
	}
}

// TestDerivedEffectiveGenerationNeverReusesTheUpstreamIdentity proves the
// identity rule of a locally enriched hop. Two payloads that share one identity
// would let a downstream treat different catalogs as one generation.
func TestDerivedEffectiveGenerationNeverReusesTheUpstreamIdentity(t *testing.T) {
	t.Parallel()
	const upstream = "generation-upstream"
	first := deriveEffectiveGenerationID(upstream, "sha256:aaaaaaaaaaaaaaaabbbb")
	second := deriveEffectiveGenerationID(upstream, "sha256:ccccccccccccccccdddd")
	if first == upstream || second == upstream {
		t.Fatalf("derived identities %q and %q reuse the upstream identity",
			first, second)
	}
	if first == second {
		t.Fatalf("two payloads share the identity %q", first)
	}
	if !strings.HasPrefix(first, upstream+effectiveGenerationLocalSuffix) {
		t.Fatalf("derived identity = %q, want the upstream identity plus a local suffix",
			first)
	}
	if repeated := deriveEffectiveGenerationID(
		upstream, "sha256:aaaaaaaaaaaaaaaabbbb"); repeated != first {
		t.Fatalf("second derivation = %q, want the stable identity %q", repeated, first)
	}
	if empty := deriveEffectiveGenerationID(upstream, ""); empty != upstream+effectiveGenerationLocalSuffix+"local" {
		t.Fatalf("derived identity with no digest = %q, want a named fallback", empty)
	}
}
