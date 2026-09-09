package update

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

func TestSourceFailureSummaryNamesOnlyBoundedValues(t *testing.T) {
	var out bytes.Buffer
	result := &sync.Result{Partial: true, SourceFailures: []sources.SourceFailure{
		{Source: sources.ModelsDevGitID, Reason: sources.ProviderReasonDependencyUnavailable},
		{Source: "secret-source", Reason: sources.ProviderReasonDependencyUnavailable},
		{Source: sources.ModelsDevHTTPID, Reason: "secret-reason"},
	}}
	if err := displaySourceFailures(&out, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Catalog acquisition is partial.") || !strings.Contains(out.String(), "models_dev_git: dependency_unavailable") || strings.Contains(out.String(), "secret") {
		t.Fatalf("unsafe or incomplete source summary: %s", out.String())
	}
	out.Reset()
	if err := displaySourceFailures(&out, &sync.Result{}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("complete acquisition produced a partial warning")
	}
}
