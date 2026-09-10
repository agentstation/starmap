package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCanonicalAliasPreservesScopedAbsenceAndOperatorExclusions(t *testing.T) {
	generation := aliasGeneration(t, "alias-scopes", activeAlias("author/old"), activeAlias("author/other"))
	base, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
		"wire": {ID: "wire", Name: "Model", ModelRef: "author/current"},
	}}); err != nil {
		t.Fatal(err)
	}
	base, err = builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	generation.Payload, err = catalogs.EncodeCatalogPayload(base)
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
	generation.Manifest.SourceObservations[0].EvidenceChecksum = generation.Manifest.Payload.Checksum
	generation.Manifest.SourceObservations[0].Revision.Value = generation.Manifest.Payload.Checksum
	if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
		t.Fatal(err)
	}
	first := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: "first", Revision: "1", ProviderID: "provider", AccountID: "account-a",
		Region: "global", APISurface: "models.list", MembershipAuthority: sources.ProviderMembershipScope,
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "public",
	}
	second := first
	second.ID, second.AccountID = "second", "account-b"
	source := newStubSource("alias-scope-baseline")
	source.replies = []SourceRead{aliasRead(generation)}
	options := []Option{WithSource(source), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithStateDirectory(privateRuntimeDirectory(t)), WithProviderBindings(first, second)}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	observation := func(binding sources.ProviderAcquisitionBinding, present bool, at time.Time) sources.Observation {
		t.Helper()
		b := catalogs.NewEmpty()
		provider := catalogs.Provider{ID: "provider", Name: "Provider"}
		if present {
			provider.Models = map[string]*catalogs.Model{"wire": {ID: "wire", Name: "Model", ModelRef: "author/current"}}
		}
		if err := b.SetProvider(provider); err != nil {
			t.Fatal(err)
		}
		facts, err := catalogs.NewObservationCatalog(b)
		if err != nil {
			t.Fatal(err)
		}
		return providerResetObservation(t, sources.ProvidersID, facts, at, &binding)
	}
	at := generation.Manifest.GeneratedAt.Add(time.Minute)
	if _, err := connected.PublishObservations(t.Context(), observation(first, false, at), observation(second, true, at)); err != nil {
		t.Fatal(err)
	}
	assert := func(r *Runtime, scopedRemoved, canonicalRemoved bool) {
		t.Helper()
		state := r.State()
		for _, id := range []string{"author/old", "author/other"} {
			definition, err := state.Catalog.FindModel(id)
			if err != nil || definition.ID != "author/current" {
				t.Fatalf("alias %s: %+v, %v", id, definition, err)
			}
			offerings, err := state.Catalog.DefinitionOfferings(definition.ID)
			if err != nil || len(offerings) != 1 {
				t.Fatalf("visible offerings: %v, %v", offerings, err)
			}
			if state.Catalog.Removals().ContainsCanonical(definition.ID) != canonicalRemoved {
				t.Fatal("alias changed canonical exclusion")
			}
			if state.Catalog.Removals().ContainsAlias(catalogs.ModelDefinitionID(id)) {
				t.Fatal("entry removal became alias removal")
			}
			scopes := state.Catalog.MembershipScopes()
			if len(scopes) != 2 {
				t.Fatalf("scopes=%d", len(scopes))
			}
			for _, scope := range scopes {
				key := catalogs.MembershipScopeKey{PublisherID: scope.PublisherID, BindingID: scope.BindingID, BindingRevision: scope.BindingRevision}
				present, known := state.Catalog.ScopeMembership(key, offerings[0].ProviderID, string(offerings[0].ProviderModelID))
				if !known || present != (scope.AccountID == second.AccountID) {
					t.Fatal("alias changed scoped availability")
				}
				wantRemoval := scopedRemoved && scope.AccountID == first.AccountID
				if state.Catalog.Removals().ContainsScoped(scope, offerings[0].ProviderModelID) != wantRemoval {
					t.Fatal("alias widened an account exclusion")
				}
			}
		}
	}
	assert(connected, false, false)
	var scope catalogs.ProviderMembershipScope
	for _, candidate := range connected.Catalog().MembershipScopes() {
		if candidate.AccountID == first.AccountID {
			scope = candidate
		}
	}
	scoped, err := catalogs.NewScopedRemovalTarget(scope, "wire")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), scoped); err != nil {
		t.Fatal(err)
	}
	assert(connected, true, false)
	canonical, err := catalogs.NewCanonicalRemovalTarget("author/current")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), scoped, canonical); err != nil {
		t.Fatal(err)
	}
	assert(connected, true, true)
	accepted := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	assert(restarted, true, true)
	if restarted.State().GenerationID != accepted.GenerationID {
		t.Fatal("restart changed exclusion generation")
	}
	if _, err := restarted.ReplaceRemovalTargets(t.Context(), restarted.State(), scoped); err != nil {
		t.Fatal(err)
	}
	assert(restarted, true, false)
}
