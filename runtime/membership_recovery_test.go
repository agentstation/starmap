package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicMembershipAbsenceSurvivesPartialRefreshAndRestart(t *testing.T) {
	binding := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion,
		ID:            "public-inventory", Revision: "1", ProviderID: "provider",
		Public: true, Region: "global", APISurface: "models.list",
		MembershipAuthority: sources.ProviderMembershipProvider,
		CredentialRole:      sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "public",
	}
	store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	connected, options, at := providerResetRuntime(t, store, WithProviderBindings(binding))
	baseline := connected.State().Catalog
	builder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models = nil
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	empty, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	removal := providerResetObservation(t, sources.ProvidersID, empty, at, &binding)
	if _, err := connected.PublishObservations(t.Context(), removal); err != nil {
		t.Fatal(err)
	}
	assertAbsent := func(stage string, runtime *Runtime) {
		t.Helper()
		provider, err := runtime.State().Catalog.Provider("provider")
		if err != nil {
			t.Fatal(err)
		}
		if provider.Models["model"] == nil {
			t.Errorf("%s deleted a visible catalog offering", stage)
		}
		key := catalogs.MembershipScopeKey{PublisherID: runtime.layers.publisherID, BindingID: binding.ID, BindingRevision: binding.Revision}
		present, known := runtime.Catalog().ScopeMembership(key, "provider", "model")
		if present || !known {
			t.Errorf("%s availability = %t/%t, want absent/known", stage, present, known)
		}
		generation, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		bound := false
		for _, link := range generation.Manifest.SourceObservations {
			bound = bound || link.ObservationID == removal.ID
		}
		if !bound {
			t.Errorf("%s lost the removal receipt", stage)
		}
	}
	assertAbsent("complete inventory", connected)
	partial, err := sources.NewObservation(sources.ProvidersID, empty, sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: at.Add(time.Minute),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeFetchFailed, Message: "Provider inventory is incomplete."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), partial); err != nil {
		t.Fatal(err)
	}
	assertAbsent("partial inventory", connected)

	payload, err := catalogs.EncodeCatalogPayload(baseline)
	if err != nil {
		t.Fatal(err)
	}
	reply := testSourceRead(t, "baseline-after-removal", payload, at.Add(2*time.Minute))
	source := connected.source.(*stubSource)
	source.mu.Lock()
	source.replies = []SourceRead{reply}
	source.mu.Unlock()
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertAbsent("refreshed baseline", connected)
	before := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	assertAbsent("restart", reopened)
	if reopened.State().GenerationID != before.GenerationID || reopened.State().PayloadChecksum != before.PayloadChecksum {
		t.Error("restart changed accepted membership state")
	}

	replacement := providerResetObservation(t, sources.ProvidersID, baseline, at.Add(2*time.Minute), &binding)
	reset, err := reopened.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{replacement}, nil
	}, ObservationReset{ProviderID: "provider", BindingID: binding.ID, BindingRevision: binding.Revision})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reset.Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Models["model"] == nil {
		t.Fatal("reset retained discarded membership removal")
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	afterReset := openTestRuntime(t, options...)
	restored, err = afterReset.State().Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range afterReset.layers.buildEvidence.SourceObservations {
		if link.ObservationID == removal.ID {
			t.Fatal("reset retained active removal evidence")
		}
	}
	if restored.Models["model"] == nil || afterReset.State().GenerationID != reset.GenerationID {
		t.Fatal("restart restored discarded removal or changed reset generation")
	}
}

func TestPublicMembershipWithdrawalPreservesAccountAcrossRestart(t *testing.T) {
	public := sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: "public", Revision: "1", ProviderID: "provider",
		Public: true, Region: "global", APISurface: "models.list", MembershipAuthority: sources.ProviderMembershipProvider,
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "public",
	}
	account := public
	account.ID, account.AccountID, account.Public, account.MembershipAuthority = "account", "account", false, sources.ProviderMembershipScope
	store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	connected, options, at := providerResetRuntime(t, store, WithProviderBindings(public, account))
	baseline := connected.State().Catalog
	provider, err := baseline.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models = nil
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	empty, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	positive := providerResetObservation(t, sources.ProvidersID, baseline, at, &account)
	removal := providerResetObservation(t, sources.ProvidersID, empty, at.Add(time.Minute), &public)
	if _, err := connected.PublishObservations(t.Context(), positive); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), removal); err != nil {
		t.Fatal(err)
	}
	assertPresent := func(runtime *Runtime, want bool) {
		t.Helper()
		provider, err := runtime.State().Catalog.Provider("provider")
		if err != nil {
			t.Fatal(err)
		}
		if got := provider.Models["model"] != nil; got != want {
			t.Fatalf("membership present=%t, want %t", got, want)
		}
	}
	assertPresent(connected, true)
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	assertPresent(reopened, true)
	withdrawn := providerResetObservation(t, sources.ProvidersID, empty, at.Add(2*time.Minute), &account)
	if _, err := reopened.PublishObservations(t.Context(), withdrawn); err != nil {
		t.Fatal(err)
	}
	assertPresent(reopened, true)
	for _, binding := range []sources.ProviderAcquisitionBinding{public, account} {
		key := catalogs.MembershipScopeKey{PublisherID: reopened.layers.publisherID, BindingID: binding.ID, BindingRevision: binding.Revision}
		present, known := reopened.Catalog().ScopeMembership(key, "provider", "model")
		if present || !known {
			t.Fatalf("scope %s did not retain observed absence", binding.ID)
		}
	}
	generation := reopened.State().GenerationID
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	afterWithdrawal := openTestRuntime(t, options...)
	assertPresent(afterWithdrawal, true)
	if afterWithdrawal.State().GenerationID != generation {
		t.Fatal("restart changed withdrawn membership generation")
	}

	if err := afterWithdrawal.Close(); err != nil {
		t.Fatal(err)
	}
	withoutPublicAuthority := openTestRuntime(t, append(options, WithProviderBindings(account))...)
	assertPresent(withoutPublicAuthority, true)
	for _, link := range withoutPublicAuthority.layers.buildEvidence.SourceObservations {
		if link.ObservationID == removal.ID {
			t.Fatal("revoked binding retained active removal evidence")
		}
	}
}
