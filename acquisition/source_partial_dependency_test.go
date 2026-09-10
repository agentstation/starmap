package acquisition

import (
	"encoding/json"
	"errors"
	"github.com/agentstation/starmap"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestSourceAcquirerPreservesPartialDependencyFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	current := acquisitionTestCatalog(t)
	workspace, err := catalogs.NewBuilderFrom(current)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "workspace")
	if err := workspace.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.LocalCatalogID, sources.ModelsDevGitID), pkgsync.WithCatalogPath(path), pkgsync.WithModelsDevGitCommit(strings.Repeat("a", 40)), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "checkout")), pkgsync.WithSkipDepPrompts(true))
	if err != nil {
		t.Fatal(err)
	}
	observations, err := acquirer.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: current})
	var dependency *pkgerrors.DependencyError
	if !errors.As(err, &dependency) || dependency.Source != string(sources.ModelsDevGitID) {
		t.Errorf("partial acquisition lost typed dependency cause: %v", err)
	}
	if sources.ClassifyProviderReason(err) != sources.ProviderReasonDependencyUnavailable {
		t.Errorf("partial failure reason=%s", sources.ClassifyProviderReason(err))
	}
	if len(observations) != 1 || observations[0].SourceID != sources.LocalCatalogID {
		t.Fatalf("partial acquisition lost the available local observation: %v", observations)
	}
}

func TestManualPartialDependencyResultReportsFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	current := acquisitionTestCatalog(t)
	workspace, err := catalogs.NewBuilderFrom(current)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "workspace")
	if err := workspace.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	client, err := starmap.New(starmap.WithCatalogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	syncer, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	result, err := syncer.Sync(t.Context(), pkgsync.WithDryRun(true), pkgsync.WithSources(sources.LocalCatalogID, sources.ModelsDevGitID), pkgsync.WithCatalogPath(path), pkgsync.WithModelsDevGitCommit(strings.Repeat("a", 40)), pkgsync.WithSkipDepPrompts(true))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Partial  bool `json:"partial"`
		Failures []struct {
			Source sources.ID             `json:"source"`
			Reason sources.ProviderReason `json:"reason"`
		} `json:"source_failures"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if !report.Partial || len(report.Failures) != 1 || report.Failures[0].Source != sources.ModelsDevGitID || report.Failures[0].Reason != sources.ProviderReasonDependencyUnavailable {
		t.Fatalf("manual result hides partial source failure: %+v", report)
	}
}
