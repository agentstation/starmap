package runtime

import (
	"reflect"
	"testing"
	"time"
)

func TestSourcePolicyResolutionMatchesRuntimeOptions(t *testing.T) {
	aliases := []string{"second-address"}
	input := DefaultSourcePolicy()
	input.Kind, input.URL, input.StartupPolicy = SourceStarmap, "https://authority.example", StartupRequireAuthority
	input.AuthorityID, input.PolicyID, input.Aliases = "enterprise", "production", aliases
	options := []Option{WithSourcePolicy(input), WithSourcePollInterval(0), WithSourceMaxAge(2 * time.Hour)}
	expected := defaults()
	if _, err := expected.apply(options...); err != nil {
		t.Fatal(err)
	}
	expected.resolve()
	actual, err := ResolveSourcePolicy(options...)
	if err != nil || !reflect.DeepEqual(actual, expected.source) {
		t.Fatal("resolved policy differs from runtime", err)
	}
	actual.Aliases[0] = "caller-change"
	if aliases[0] != "second-address" || expected.source.Aliases[0] != "second-address" {
		t.Fatal("resolved policy aliases share caller storage")
	}
	unchanged, err := ResolveSourcePolicy(options...)
	if err != nil || !reflect.DeepEqual(unchanged, expected.source) {
		t.Fatal("caller mutation changed later policy resolution", err)
	}
}

func TestSourcePolicyResolutionDefaultsAndRefusals(t *testing.T) {
	actual, err := ResolveSourcePolicy()
	if err != nil || !reflect.DeepEqual(actual, DefaultSourcePolicy()) {
		t.Fatal("source defaults changed", err)
	}
	for _, options := range [][]Option{{nil}, {WithCatalogSource("unknown")}, {WithSourceStartupPolicy(string(StartupRequireAuthority))}} {
		if _, err := ResolveSourcePolicy(options...); err == nil {
			t.Fatal("invalid source options passed")
		}
	}
}

func TestSourcePolicyResolutionDoesNotStartExternalRoles(t *testing.T) {
	calls := 0
	option := func(config *options) error {
		config.sourceAPIKey = "opaque-test-value"
		config.now = func() time.Time { calls++; return time.Time{} }
		return nil
	}
	if _, err := ResolveSourcePolicy(option, WithStateDirectory("/unavailable-source-policy-resolution")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("policy resolution called the runtime clock")
	}
}
