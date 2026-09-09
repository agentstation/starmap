package acquisition

import (
	"context"
	stderrors "errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestSourceAcquirerConstructorIsPassiveAndValidatesSelection(t *testing.T) {
	root := t.TempDir()
	selected := []sources.ID{sources.ModelsDevHTTPID}
	acquirer, err := NewSourceAcquirer(pkgsync.WithSourcesDir(filepath.Join(root, "cache")), func(options *pkgsync.Options) { options.Sources = selected })
	if err != nil {
		t.Fatal(err)
	}
	selected[0] = sources.ProvidersID
	if !slices.Equal(acquirer.options.Sources, []sources.ID{sources.ModelsDevHTTPID}) {
		t.Fatal("constructor retained caller-owned source slice")
	}
	for _, test := range []struct {
		name    string
		options []pkgsync.Option
	}{
		{"providers", []pkgsync.Option{pkgsync.WithSources(sources.ProvidersID)}},
		{"baseline", []pkgsync.Option{pkgsync.WithSources(sources.EmbeddedCatalogID)}},
		{"artifact", []pkgsync.Option{pkgsync.WithSources(sources.ReleaseArtifactID)}},
		{"unknown", []pkgsync.Option{pkgsync.WithSources("unknown")}},
		{"both_metadata_forms", []pkgsync.Option{pkgsync.WithSources(sources.ModelsDevHTTPID, sources.ModelsDevGitID)}},
		{"fresh", []pkgsync.Option{pkgsync.WithFresh(true)}},
		{"dry_run", []pkgsync.Option{pkgsync.WithDryRun(true)}},
		{"fixed_provider", []pkgsync.Option{pkgsync.WithProvider("openai")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSourceAcquirer(test.options...)
			var invalid *pkgerrors.ValidationError
			if !stderrors.As(err, &invalid) {
				t.Fatalf("invalid selection = %v", err)
			}
		})
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("constructor wrote source files: %v, %v", entries, err)
	}
}

func TestSourceAcquirerPreservesProviderFiltersAndHTTPReceipts(t *testing.T) {
	payload, err := embedded.FS.ReadFile("sources/models.dev/api.json")
	if err != nil {
		t.Fatal(err)
	}
	transport := &sourcePathTransport{testing: t, payload: payload}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevHTTPID), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "cache")))
	if err != nil {
		t.Fatal(err)
	}
	if transport.calls != 0 {
		t.Fatal("constructor called HTTP")
	}
	providers := []catalogs.ProviderID{"openai", "anthropic", "openai"}
	observations, err := acquirer.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t), Providers: providers})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(providers, []catalogs.ProviderID{"openai", "anthropic", "openai"}) {
		t.Fatal("acquisition changed caller filters")
	}
	if len(observations) != 2 {
		t.Fatalf("observations = %d, want two provider scopes", len(observations))
	}
	for index, provider := range []catalogs.ProviderID{"anthropic", "openai"} {
		observation := observations[index]
		if observation.SourceID != sources.ModelsDevHTTPID || observation.ID == "" || observation.EvidenceChecksum == "" {
			t.Fatalf("source receipt = %+v", observation.Link())
		}
		if _, err := observation.Receipt(); err != nil {
			t.Fatal(err)
		}
		list := observation.Catalog.Providers().List()
		if len(list) != 1 || list[0].ID != provider {
			t.Fatalf("observation %d escaped provider filter %s", index, provider)
		}
	}
	if transport.calls == 0 {
		t.Fatal("explicit acquisition did not read HTTP")
	}
}

func TestSourceAcquirerReportsMissingDependencies(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevGitID), pkgsync.WithModelsDevGitCommit(strings.Repeat("a", 40)), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "checkout")), pkgsync.WithSkipDepPrompts(true))
	if err != nil {
		t.Fatal(err)
	}
	observations, err := acquirer.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t)})
	var configuration *pkgerrors.ConfigError
	if !stderrors.As(err, &configuration) || !strings.Contains(err.Error(), string(sources.ModelsDevGitID)) {
		t.Fatalf("missing selected dependency = %v", err)
	}
	if len(observations) != 0 {
		t.Fatal("missing metadata source returned baseline as acquisition")
	}
}

func TestSourceAcquirerMissingWorkspaceIsNotAResult(t *testing.T) {
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.LocalCatalogID), pkgsync.WithCatalogPath(filepath.Join(t.TempDir(), "absent")))
	if err != nil {
		t.Fatal(err)
	}
	observations, err := acquirer.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t)})
	if err != nil || len(observations) != 0 {
		t.Fatalf("absent optional workspace = %v, %v", observations, err)
	}
}

func TestSourceAcquirerRejectsInvalidRequestAndCancellation(t *testing.T) {
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.LocalCatalogID))
	if err != nil {
		t.Fatal(err)
	}
	current := acquisitionTestCatalog(t)
	for _, request := range []runtime.SourceAcquisitionRequest{{}, {Current: current, Providers: []catalogs.ProviderID{" "}}} {
		if _, err := acquirer.AcquireSources(t.Context(), request); err == nil {
			t.Fatal("invalid request succeeded")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := acquirer.AcquireSources(ctx, runtime.SourceAcquisitionRequest{Current: current}); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled request = %v", err)
	}
}

func TestSourceAcquirerExplicitRequestReplacesConstructorDefaults(t *testing.T) {
	transport := &sourcePathTransport{testing: t}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevHTTPID), pkgsync.WithCatalogPath(filepath.Join(t.TempDir(), "absent")), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "cache")))
	if err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]sources.ID{{sources.LocalCatalogID}, {}} {
		observations, err := acquirer.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t), Sources: ids})
		if err != nil || len(observations) != 0 {
			t.Fatalf("request %v produced %d observations, error %v", ids, len(observations), err)
		}
	}
	if transport.calls != 0 {
		t.Fatal("excluded HTTP source was read")
	}
}

func TestSourceAcquirerGitPinOverrideAndClear(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	pin := strings.Repeat("a", 40)
	empty := ""
	for _, test := range []struct {
		name, initial string
		requested     *string
		pinError      bool
	}{
		{name: "constructor", initial: pin},
		{name: "request", initial: "invalid", requested: &pin},
		{name: "clear", initial: pin, requested: &empty, pinError: true},
		{name: "missing", pinError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			collector, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevGitID), pkgsync.WithModelsDevGitCommit(test.initial), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "checkout")), pkgsync.WithSkipDepPrompts(true))
			if err != nil {
				t.Fatal(err)
			}
			observations, err := collector.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t), ModelsDevGitCommit: test.requested})
			var validation *pkgerrors.ValidationError
			pinFailure := stderrors.As(err, &validation) && validation.Field == "ModelsDevGitCommit"
			if pinFailure != test.pinError {
				t.Fatalf("pin failure=%t, error=%v", pinFailure, err)
			}
			if !test.pinError {
				var configuration *pkgerrors.ConfigError
				if !stderrors.As(err, &configuration) {
					t.Fatalf("valid pin did not reach dependency validation: %v", err)
				}
			}
			if len(observations) != 0 {
				t.Fatal("failed Git input became an observation")
			}
		})
	}
	collector, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevGitID), pkgsync.WithModelsDevGitCommit(pin), pkgsync.WithCatalogPath(filepath.Join(t.TempDir(), "absent")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = collector.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t), Sources: []sources.ID{sources.LocalCatalogID}}); err != nil {
		t.Fatalf("local source inherited a Git-only pin: %v", err)
	}
}
