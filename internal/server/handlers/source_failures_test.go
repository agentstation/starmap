package handlers

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestUpdateDetailPreservesPartialSourceFailure(t *testing.T) {
	failures := []sources.SourceFailure{{Source: sources.ModelsDevGitID, Reason: sources.ProviderReasonDependencyUnavailable}, {Source: "secret-source", Reason: "secret-reason"}}
	h := &Handlers{app: &testApplication{SyncFunc: func(context.Context, ...pkgsync.Option) (*pkgsync.Result, error) {
		return &pkgsync.Result{Partial: true, SourceFailures: failures}, nil
	}}}
	detail, err := h.runCatalogUpdate(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	actual, ok := detail["source_failures"].([]sources.SourceFailure)
	if detail["partial"] != true || !ok || len(actual) != 1 || actual[0] != failures[0] {
		t.Fatalf("partial update detail=%v", detail)
	}
	failures[0].Source = sources.LocalCatalogID
	if actual[0].Source != sources.ModelsDevGitID {
		t.Fatal("update detail aliases acquisition results")
	}
}
