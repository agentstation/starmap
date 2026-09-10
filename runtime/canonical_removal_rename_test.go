package runtime

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCanonicalRemovalSurvivesRenameAndRestartUntilRestore(t *testing.T) {
	initial := aliasGeneration(t, "before-rename")
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel("author", catalogs.Model{ID: "old", Name: "Model", Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
		t.Fatal(err)
	}
	var err error
	initial.Payload, err = catalogs.EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	initial.Manifest.Payload = catalogs.DescribeCatalogPayload(initial.Payload)
	initial.Manifest.SourceObservations[0].EvidenceChecksum = initial.Manifest.Payload.Checksum
	initial.Manifest.SourceObservations[0].Revision.Value = initial.Manifest.Payload.Checksum
	if _, err := catalogs.DecodeCatalogGeneration(initial); err != nil {
		t.Fatal(err)
	}
	source := newStubSource("canonical-removal-baseline")
	source.replies = []SourceRead{aliasRead(initial)}
	options := []Option{WithSource(source), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithStateDirectory(privateRuntimeDirectory(t))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	target, err := catalogs.NewCanonicalRemovalTarget("author/old")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), target); err != nil {
		t.Fatal(err)
	}
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "after-rename", activeAlias("author/old")))}
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertRemoved := func(r *Runtime, removed bool) {
		t.Helper()
		definition, err := r.Catalog().FindModel("author/old")
		if err != nil || definition.ID != "author/current" {
			t.Fatalf("renamed identity: %s, %v", definition.ID, err)
		}
		if r.Catalog().Removals().ContainsCanonical(definition.ID) != removed {
			t.Fatal("renamed model lost the operator's removal state")
		}
	}
	assertRemoved(connected, true)
	accepted := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	assertRemoved(restarted, true)
	if restarted.State().GenerationID != accepted.GenerationID {
		t.Fatal("restart changed the accepted removal generation")
	}
	if _, err := restarted.ReplaceRemovalTargets(t.Context(), restarted.State()); err != nil {
		t.Fatal(err)
	}
	assertRemoved(restarted, false)
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	assertRemoved(openTestRuntime(t, options...), false)
}
