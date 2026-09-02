package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// TestAcquisitionPolicyFromEnabledAndInterval proves that scheduled
// acquisition holds exactly one switch and one period. No start-time setting
// and no mode setting exists, so no deployment selects a hidden behavior.
func TestAcquisitionPolicyFromEnabledAndInterval(t *testing.T) {
	t.Parallel()

	policyType := reflect.TypeFor[AcquisitionPolicy]()
	names := make([]string, 0, policyType.NumField())
	for i := range policyType.NumField() {
		names = append(names, policyType.Field(i).Name)
	}
	if want := []string{"Enabled", "Interval"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("AcquisitionPolicy fields = %v, want %v", names, want)
	}

	policy := DefaultAcquisitionPolicy()
	if !policy.Enabled {
		t.Error("the default acquisition policy must run acquisition")
	}
	if policy.Interval != DefaultAcquisitionInterval {
		t.Errorf("default interval = %s, want %s", policy.Interval, DefaultAcquisitionInterval)
	}
	if err := policy.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	// A disabled policy needs no period, because nothing runs.
	if err := (AcquisitionPolicy{}).Validate(); err != nil {
		t.Errorf("a disabled policy must validate: %v", err)
	}
	// An enabled policy without a period is a configuration error.
	if err := (AcquisitionPolicy{Enabled: true}).Validate(); err == nil {
		t.Error("an enabled policy without an interval must fail")
	}

	// The two options build exactly this policy.
	config, err := defaults().apply(
		WithAcquisitionEnabled(true),
		WithAcquisitionInterval(90*time.Minute),
	)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := AcquisitionPolicy{Enabled: true, Interval: 90 * time.Minute}
	if config.runtime.acquisition != want {
		t.Errorf("acquisition policy = %+v, want %+v", config.runtime.acquisition, want)
	}
}

// TestSourceSelectionNames proves that source selection accepts the five
// documented names and rejects every other name with a typed error.
func TestSourceSelectionNames(t *testing.T) {
	t.Parallel()

	accepted := []SourceKind{SourcePublic, SourceGitHub, SourceStarmap, SourceFile, SourceEmbedded}
	if got := SourceKinds(); !reflect.DeepEqual(got, accepted) {
		t.Fatalf("SourceKinds() = %v, want %v", got, accepted)
	}
	for _, kind := range accepted {
		parsed, err := ParseSourceKind(string(kind))
		if err != nil {
			t.Errorf("ParseSourceKind(%q): %v", kind, err)
			continue
		}
		if parsed != kind {
			t.Errorf("ParseSourceKind(%q) = %q", kind, parsed)
		}
	}

	rejected := []string{"", "Public", "gitlab", "public ", "starmap-cascade"}
	for _, name := range rejected {
		_, err := ParseSourceKind(name)
		if err == nil {
			t.Errorf("ParseSourceKind(%q) accepted an unknown name", name)
			continue
		}
		var validation *errors.ValidationError
		if !stderrors.As(err, &validation) {
			t.Errorf("ParseSourceKind(%q) error = %T, want *errors.ValidationError", name, err)
		}
	}

	// Only the public channel is not deployment-owned.
	if SourcePublic.Custom() {
		t.Error("the public channel must not count as a custom source")
	}
	for _, kind := range []SourceKind{SourceGitHub, SourceStarmap, SourceFile, SourceEmbedded} {
		if !kind.Custom() {
			t.Errorf("%q must count as a custom source", kind)
		}
	}
}

// TestCustomSourceNeverFallsBackToPublic proves that a named source is
// terminal. A misconfigured custom source fails instead of reading the public
// GitHub channel, so no deployment reaches a source it did not name.
func TestCustomSourceNeverFallsBackToPublic(t *testing.T) {
	t.Parallel()

	t.Run("cascade without an injected client fails", func(t *testing.T) {
		t.Parallel()
		runtime, err := Open(context.Background(),
			WithStateDirectory(t.TempDir()),
			WithCatalogSource("starmap"),
			WithSourceURL("https://catalog.example"),
			WithStartupSpread(0),
			WithAcquisitionEnabled(false),
		)
		if err == nil {
			t.Fatalf("Open selected the fallback source %q", runtime.source.Identity())
		}
		var config *errors.ConfigError
		if !stderrors.As(err, &config) {
			t.Fatalf("error = %T, want *errors.ConfigError", err)
		}
	})

	t.Run("file source without a path fails", func(t *testing.T) {
		t.Parallel()
		runtime, err := Open(context.Background(),
			WithStateDirectory(t.TempDir()),
			WithCatalogSource("file"),
			WithStartupSpread(0),
			WithAcquisitionEnabled(false),
		)
		if err == nil {
			t.Fatalf("Open selected the fallback source %q", runtime.source.Identity())
		}
	})

	t.Run("file source reads the named file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "catalog.json")
		payload := testCatalogPayload(t, "file-provider", "file-model", "File Model")
		if err := os.WriteFile(path, payload, constants.SecureFilePermissions); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		runtime := openTestRuntime(t, WithCatalogSource("file"), WithSourceURL(path))
		if got := runtime.source.Identity(); got != string(SourceFile) {
			t.Fatalf("source identity = %q, want %q", got, SourceFile)
		}
		report, err := runtime.RefreshSource(context.Background())
		if err != nil {
			t.Fatalf("RefreshSource: %v", err)
		}
		if !report.Changed || !report.Published {
			t.Fatalf("report = %+v, want a published change", report)
		}
		if _, found := runtime.Catalog().Providers().Get("file-provider"); !found {
			t.Error("the effective catalog lost the file provider")
		}
	})

	t.Run("github source keeps the named channel", func(t *testing.T) {
		t.Parallel()
		runtime := openTestRuntime(t,
			WithCatalogSource("github"),
			WithSourceRepository("example/catalog"),
			WithSourceChannel("catalog-stable"),
		)
		identity := runtime.source.Identity()
		want := "github:example/catalog#catalog-stable"
		if identity != want {
			t.Errorf("source identity = %q, want %q", identity, want)
		}
		if public := DefaultSourcePolicy().SafeIdentity(); identity == public {
			t.Errorf("the named channel adopted the public identity %q", public)
		}
	})
}
