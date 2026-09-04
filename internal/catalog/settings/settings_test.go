package settings_test

import (
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// sampleValues holds one valid value for every canonical catalog setting. A new
// canonical name without an entry here fails TestEveryCanonicalNameMapsToOption.
func sampleValues() map[string]string {
	return map[string]string{
		settings.Source:               "embedded",
		settings.SourceURL:            "https://example.test/catalog",
		settings.SourceAPIKey:         "placeholder",
		settings.SourceRepository:     "example/catalog",
		settings.SourceChannel:        "catalog-latest",
		settings.SourceSignerWorkflow: ".github/workflows/publish.yml",
		settings.SourceToken:          "placeholder",
		settings.SourcePollInterval:   "30m",
		settings.SourceStartupPolicy:  "embedded",
		settings.SourceMaxAge:         "12h",
		settings.SourceMaxHops:        "4",
		settings.SourceAliases:        "replica-a,replica-b",
		settings.AcquisitionEnabled:   "false",
		settings.AcquisitionInterval:  "2h",
		settings.CoalesceWindow:       "45s",
		settings.WorkspacePath:        "/var/lib/starmap/catalog",
		settings.StartupSpread:        "5m",
		settings.TransferIdleTimeout:  "90s",
		settings.TransferMaxDuration:  "30m",
		settings.RefreshTimeout:       "10m",
		settings.StateDirectory:       "/var/lib/starmap/state",
		settings.SchedulerIdentity:    "replica-a",
	}
}

// TestEveryCanonicalNameMapsToOption proves that each canonical name selects
// exactly one runtime option and that the parser accepts no other name.
func TestEveryCanonicalNameMapsToOption(t *testing.T) {
	values := sampleValues()
	names := settings.Names()
	if len(names) != len(values) {
		t.Fatalf("canonical names = %d, sample values = %d", len(names), len(values))
	}
	for _, name := range names {
		if _, held := values[name]; !held {
			t.Fatalf("canonical name %q has no sample value", name)
		}
		if !strings.HasPrefix(name, settings.Prefix) {
			t.Fatalf("canonical name %q lacks the %q prefix", name, settings.Prefix)
		}
	}

	for _, name := range names {
		config, err := settings.Load(single(name, values[name]))
		if err != nil {
			t.Fatalf("Load(%s): %v", name, err)
		}
		configured := config.Configured()
		if len(configured) != 1 || configured[0] != name {
			t.Fatalf("configured for %s = %v, want exactly [%s]", name, configured, name)
		}
		if options := config.Options(); len(options) != 1 {
			t.Fatalf("options for %s = %d, want 1", name, len(options))
		}
	}

	// An unknown name supplies nothing, so the runtime keeps every default.
	config, err := settings.Load(single("STARMAP_CATALOG_UNKNOWN", "value"))
	if err != nil {
		t.Fatalf("Load(unknown): %v", err)
	}
	if configured := config.Configured(); len(configured) != 0 {
		t.Fatalf("configured for an unknown name = %v, want none", configured)
	}
}

// TestFlagsMirrorCanonicalNames proves that every setting carries one flag of
// the same name in kebab case.
func TestFlagsMirrorCanonicalNames(t *testing.T) {
	set := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := settings.RegisterFlags(set); err != nil {
		t.Fatalf("RegisterFlags: %v", err)
	}
	names := settings.Names()
	flags := settings.Flags()
	if len(names) != len(flags) {
		t.Fatalf("names = %d, flags = %d", len(names), len(flags))
	}
	for index, name := range names {
		want := strings.ToLower(strings.ReplaceAll(
			strings.TrimPrefix(name, settings.Prefix), "_", "-"))
		if flags[index] != want {
			t.Fatalf("flag for %s = %q, want %q", name, flags[index], want)
		}
		if set.Lookup(want) == nil {
			t.Fatalf("flag %q is not registered", want)
		}
	}
}

// TestFlagOverridesEnvironment proves that a changed flag replaces the exported
// value and that an untouched flag keeps it.
func TestFlagOverridesEnvironment(t *testing.T) {
	set := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := settings.RegisterFlags(set); err != nil {
		t.Fatalf("RegisterFlags: %v", err)
	}
	environment := single(settings.SourceChannel, "catalog-stable")

	config, err := settings.Load(settings.Chain(settings.FlagLookup(set), environment))
	if err != nil {
		t.Fatalf("Load before parse: %v", err)
	}
	if configured := config.Configured(); len(configured) != 1 {
		t.Fatalf("configured before parse = %v, want the environment value", configured)
	}

	if err := set.Parse([]string{"--catalog-source", "embedded"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	config, err = settings.Load(settings.Chain(settings.FlagLookup(set), environment))
	if err != nil {
		t.Fatalf("Load after parse: %v", err)
	}
	if config.SourceKind != runtime.SourceEmbedded {
		t.Fatalf("source kind = %q, want %q", config.SourceKind, runtime.SourceEmbedded)
	}
	if configured := config.Configured(); len(configured) != 2 {
		t.Fatalf("configured after parse = %v, want the flag and the environment", configured)
	}
}

// TestDefaultSourceIsPublic proves that a process without a catalog setting
// selects the public channel.
func TestDefaultSourceIsPublic(t *testing.T) {
	config, err := settings.Load(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if config.SourceKind != runtime.SourcePublic {
		t.Fatalf("default source = %q, want %q", config.SourceKind, runtime.SourcePublic)
	}
	if configured := config.Configured(); len(configured) != 0 {
		t.Fatalf("configured = %v, want none", configured)
	}
}

// TestConfiguredSourceReplacesTheDefault proves that each supported source kind
// replaces the public default.
func TestConfiguredSourceReplacesTheDefault(t *testing.T) {
	for _, kind := range []runtime.SourceKind{
		runtime.SourceGitHub, runtime.SourceFile, runtime.SourceEmbedded,
	} {
		config, err := settings.Load(single(settings.Source, string(kind)))
		if err != nil {
			t.Fatalf("Load(%s): %v", kind, err)
		}
		if config.SourceKind != kind {
			t.Fatalf("source kind = %q, want %q", config.SourceKind, kind)
		}
	}
}

// TestStarmapSourceComposesTheCascadeSource proves the split contract. The
// parser accepts the starmap source with its URL, and the composition step
// builds the cascade source that the root package cannot construct.
func TestStarmapSourceComposesTheCascadeSource(t *testing.T) {
	values := map[string]string{
		settings.Source:              string(runtime.SourceStarmap),
		settings.SourceURL:           "https://catalog.example.test",
		settings.StartupSpread:       "90s",
		settings.TransferIdleTimeout: "7s",
		settings.TransferMaxDuration: "3m",
	}
	config, err := settings.Load(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if config.SourceKind != runtime.SourceStarmap {
		t.Fatalf("source kind = %q, want %q", config.SourceKind, runtime.SourceStarmap)
	}
	if config.SourceURL != values[settings.SourceURL] {
		t.Fatalf("source URL = %q, want %q", config.SourceURL, values[settings.SourceURL])
	}
	// The cascade needs the canonical bounds, so the parser keeps them.
	if config.StartupSpread != 90*time.Second {
		t.Fatalf("startup spread = %s, want 90s", config.StartupSpread)
	}
	if config.TransferIdleTimeout != 7*time.Second {
		t.Fatalf("transfer idle timeout = %s, want 7s", config.TransferIdleTimeout)
	}
	if config.TransferMaxDuration != 3*time.Minute {
		t.Fatalf("transfer max duration = %s, want 3m", config.TransferMaxDuration)
	}

	options, err := settings.Composition{Config: config}.Options()
	if err != nil {
		t.Fatalf("Options: %v", err)
	}
	if len(options) == 0 {
		t.Fatal("Options returned no option for the starmap source")
	}
}

// TestStarmapSourceWithoutURLIsRejected proves that the composition step names
// the missing setting instead of opening a source with no upstream.
func TestStarmapSourceWithoutURLIsRejected(t *testing.T) {
	values := map[string]string{settings.Source: string(runtime.SourceStarmap)}
	config, err := settings.Load(func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = settings.Composition{Config: config}.Options()
	var configErr *errors.ConfigError
	if !stderrors.As(err, &configErr) {
		t.Fatalf("Options error = %v, want a ConfigError", err)
	}
	if !strings.Contains(configErr.Message, settings.SourceURL) {
		t.Fatalf("message = %q, want the missing %s", configErr.Message, settings.SourceURL)
	}
}

// TestInvalidValueReturnsTypedError proves that a bad value names its setting.
func TestInvalidValueReturnsTypedError(t *testing.T) {
	cases := map[string]string{
		settings.SourcePollInterval: "soon",
		settings.SourceMaxHops:      "many",
		settings.AcquisitionEnabled: "maybe",
		settings.Source:             "unknown",
	}
	for name, value := range cases {
		_, err := settings.Load(single(name, value))
		var validationErr *errors.ValidationError
		if !stderrors.As(err, &validationErr) {
			t.Fatalf("Load(%s=%s) error = %v, want a ValidationError", name, value, err)
		}
	}
}

// TestNilLookupIsRejected proves that Load needs a reader.
func TestNilLookupIsRejected(t *testing.T) {
	if _, err := settings.Load(nil); err == nil {
		t.Fatal("Load(nil) returned no error")
	}
	if err := settings.RegisterFlags(nil); err == nil {
		t.Fatal("RegisterFlags(nil) returned no error")
	}
}

// single returns a lookup that supplies exactly one setting.
func single(name, value string) settings.Lookup {
	return func(requested string) (string, bool) {
		if requested != name {
			return "", false
		}
		return value, true
	}
}
