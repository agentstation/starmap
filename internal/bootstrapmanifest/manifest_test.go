package bootstrapmanifest

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestScheduledGenerationManifestChangesOnlyForCanonicalPayloadChange(t *testing.T) {
	builder := catalogs.NewEmpty()
	author := catalogs.TestAuthor(t)
	if err := builder.SetAuthor(*author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	provider := catalogs.TestProvider(t)
	model := catalogs.TestModel(t)
	provider.Models = map[string]*catalogs.Model{model.ID: model}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	firstCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build first: %v", err)
	}
	firstTime := time.Date(2026, time.July, 10, 15, 0, 0, 0, time.UTC)
	first, firstReport, err := Derive(firstCatalog, nil, firstTime)
	if err != nil {
		t.Fatalf("Derive first: %v", err)
	}
	if !firstReport.Changed || first.GenerationID == "" {
		t.Fatalf("first report/manifest = %#v/%#v", firstReport, first)
	}
	unchanged, unchangedReport, err := Derive(firstCatalog, &first, firstTime.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Derive unchanged: %v", err)
	}
	if unchangedReport.Changed || unchanged != first {
		t.Fatalf("unchanged report/manifest = %#v/%#v, want exact current", unchangedReport, unchanged)
	}
	provider.Models[model.ID].Name = "Changed canonical model name"
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider changed: %v", err)
	}
	changedCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build changed: %v", err)
	}
	changed, changedReport, err := Derive(changedCatalog, &first, firstTime.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Derive changed: %v", err)
	}
	if !changedReport.Changed || changed.GenerationID == first.GenerationID ||
		changed.Payload.Checksum == first.Payload.Checksum || changed.GeneratedAt != firstTime.Add(24*time.Hour) {
		t.Fatalf("changed report/manifest = %#v/%#v", changedReport, changed)
	}
}
