package pipeline

import (
	"context"
	stderrors "errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type manualBindingClient struct {
	failed *atomic.Bool
	calls  *atomic.Int32
}

func (c manualBindingClient) ListModels(_ context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	c.calls.Add(1)
	id := string(material.Profile().ID)
	if id == "two" && c.failed.Load() {
		return nil, stderrors.New("fixture unavailable")
	}
	return []catalogs.Model{{ID: id, ModelRef: catalogs.ModelDefinitionID("author/" + id), Name: "Live " + id, Limits: &catalogs.ModelLimits{ContextWindow: 1000}}}, nil
}

func manualBindingFixture(t *testing.T) (*catalogs.Builder, []sources.ProviderAcquisitionBinding) {
	t.Helper()
	b := catalogs.NewEmpty()
	if err := b.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	credentials := testcatalog.APIKeyCredentials("FIXTURE_BINDING_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer)
	profileTemplate := credentials.Profiles[0]
	credentials.Profiles = nil
	credentials.CatalogAcquisition.Alternatives = nil
	credentials.Inference.Alternatives = nil
	models := make(map[string]*catalogs.Model)
	var bindings []sources.ProviderAcquisitionBinding
	for _, id := range []string{"one", "two"} {
		if err := b.SetAuthorModel("author", catalogs.Model{ID: id, Name: id, Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
			t.Fatal(err)
		}
		models[id] = &catalogs.Model{ID: id, ModelRef: catalogs.ModelDefinitionID("author/" + id), Name: id, Limits: &catalogs.ModelLimits{ContextWindow: 500}}
		profile := catalogs.ProviderCredentialProfileID(id)
		selectedProfile := profileTemplate
		selectedProfile.ID = profile
		credentials.Profiles = append(credentials.Profiles, selectedProfile)
		credentials.CatalogAcquisition.Alternatives = append(credentials.CatalogAcquisition.Alternatives, profile)
		credentials.Inference.Alternatives = append(credentials.Inference.Alternatives, profile)
		bindings = append(bindings, sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: id, Revision: "1", ProviderID: "provider", AccountID: "account-" + id, Region: "global", APISurface: "models", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: profile})
	}
	if err := b.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: models, Credentials: credentials, Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models", ProtocolOptions: testcatalog.OpenAIProtocolOptions()}}}); err != nil {
		t.Fatal(err)
	}
	return b, bindings
}

func manualBindingRunner(t *testing.T, builder *catalogs.Builder, bindings []sources.ProviderAcquisitionBinding, failed *atomic.Bool, resolutions, calls *atomic.Int32) *Pipeline {
	t.Helper()
	runner := NewBoundAcquisition(func(*catalogs.Provider) (sources.ProviderClient, error) {
		return manualBindingClient{failed: failed, calls: calls}, nil
	}, sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		resolutions.Add(1)
		if len(p.Credentials.CatalogAcquisition.Alternatives) != 1 {
			return sources.ProviderCredentialMaterial{}, stderrors.New("profile selection was not restricted")
		}
		for _, profile := range p.Credentials.Profiles {
			if profile.ID == p.Credentials.CatalogAcquisition.Alternatives[0] {
				return sources.NewProviderCredentialMaterial(profile, map[catalogs.ProviderCredentialFieldID]string{"api-key": "fixture"}, sources.ProviderCredentialMetadata{}), nil
			}
		}
		return sources.ProviderCredentialMaterial{}, stderrors.New("fixture profile missing")
	}), bindings)
	runner.loadEmbedded = func() (*catalogs.Builder, error) { return catalogs.NewBuilderFrom(buildCatalog(t, builder)) }
	return runner
}

func TestManualBindingsPreserveReceiptsHealthAndPreview(t *testing.T) {
	builder, bindings := manualBindingFixture(t)
	var failed atomic.Bool
	var resolutions, calls atomic.Int32
	runner := manualBindingRunner(t, builder, bindings, &failed, &resolutions, &calls)
	bindings[0].ID = "caller-mutated"
	options := []pkgsync.Option{pkgsync.WithSources(sources.ProvidersID), pkgsync.WithDryRun(true), pkgsync.WithRequireAllSources(true)}
	prepared, err := runner.Prepare(t.Context(), buildCatalog(t, builder), options...)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Publish || !prepared.Result.DryRun || prepared.Result.GenerationID != "" {
		t.Fatal("preview requested publication")
	}
	if resolutions.Load() != 2 || calls.Load() != 2 {
		t.Fatalf("calls = %d/%d", resolutions.Load(), calls.Load())
	}
	if len(prepared.Observations) != 3 || len(prepared.Result.SourceObservations) != 3 {
		t.Fatal("peer observations collapsed")
	}
	baseline := buildCatalog(t, prepared.Catalog)
	for _, observation := range prepared.Observations {
		if observation.ProviderBinding == nil {
			continue
		}
		binding := *observation.ProviderBinding
		if binding.ID == "caller-mutated" {
			t.Fatal("binding selection borrowed caller storage")
		}
		if err := observation.Validate(); err != nil {
			t.Fatal(err)
		}
		entries := baseline.Provenance().FindModel("provider", string(binding.CredentialProfileID))["limits.context_window"]
		if len(entries) == 0 {
			t.Fatal("missing provenance")
		}
		latest := currentProvenanceEntry(entries)
		if latest.ProviderBindingID != binding.ID || latest.ProviderBindingRevision != binding.Revision || latest.ObservationID != observation.ID {
			t.Fatalf("lost binding provenance: %+v", latest)
		}
	}
	failed.Store(true)
	partial, err := runner.Prepare(t.Context(), baseline, pkgsync.WithSources(sources.ProvidersID), pkgsync.WithDryRun(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range partial.Observations {
		if observation.SourceID != sources.ProvidersID {
			continue
		}
		if observation.ProviderBinding == nil {
			t.Fatal("failure lost its binding")
		}
		if observation.ProviderBinding.ID == "one" && observation.Status != sources.ObservationStatusSucceeded {
			t.Fatal("peer failure degraded healthy binding")
		}
		if observation.ProviderBinding.ID == "two" && observation.Status != sources.ObservationStatusDegraded {
			t.Fatal("provider failure was hidden")
		}
	}
	provider, _ := partial.Catalog.Provider("provider")
	if len(provider.Models) != 2 || provider.Models["two"].Limits.ContextWindow != 1000 {
		t.Fatal("peer failure discarded accepted data")
	}
	if _, err := runner.Prepare(t.Context(), baseline, options...); err == nil {
		t.Fatal("strict mode accepted a failed binding")
	}
	if _, err := runner.Prepare(t.Context(), baseline, pkgsync.WithSources(sources.ProvidersID), pkgsync.WithFresh(true)); err == nil {
		t.Fatal("fresh accepted a degraded source")
	}
}

func TestManualBindingsValidateBeforeSourceWork(t *testing.T) {
	for _, defect := range []string{"duplicate", "schema", "profile", "provider", "unbound-filter"} {
		t.Run(defect, func(t *testing.T) {
			builder, bindings := manualBindingFixture(t)
			var failed atomic.Bool
			var resolutions, calls atomic.Int32
			opts := []pkgsync.Option{pkgsync.WithSources(sources.ProvidersID)}
			switch defect {
			case "duplicate":
				bindings[1].ID = bindings[0].ID
			case "schema":
				bindings[1].SchemaVersion++
			case "profile":
				bindings[1].CredentialProfileID = "missing"
			case "provider":
				bindings[1].ProviderID = "missing"
			case "unbound-filter":
				bindings = nil
				opts = append(opts, pkgsync.WithProvider("provider"))
			}
			runner := manualBindingRunner(t, builder, bindings, &failed, &resolutions, &calls)
			runner.resolveDependencies = func(context.Context, []sources.Source, *pkgsync.Options) ([]sources.Source, error) {
				t.Error("invalid declaration reached dependencies")
				return nil, nil
			}
			if _, err := runner.Prepare(t.Context(), buildCatalog(t, builder), opts...); err == nil {
				t.Fatal("invalid declaration accepted")
			}
			if calls.Load() != 0 || resolutions.Load() != 0 {
				t.Fatal("invalid declaration acquired credentials or models")
			}
		})
	}
}

func TestManualBindingsEmptyAndSourceSelection(t *testing.T) {
	for _, selection := range []string{"empty", "embedded", "filtered"} {
		t.Run(selection, func(t *testing.T) {
			builder, bindings := manualBindingFixture(t)
			var failed atomic.Bool
			var resolutions, calls atomic.Int32
			options := []pkgsync.Option{pkgsync.WithSources(sources.ProvidersID), pkgsync.WithDryRun(true)}
			if selection == "empty" {
				bindings = nil
			}
			if selection == "embedded" {
				bindings[1].CredentialProfileID = "unavailable"
				options = []pkgsync.Option{pkgsync.WithSources(sources.EmbeddedCatalogID), pkgsync.WithDryRun(true)}
			}
			if selection == "filtered" {
				bindings[1].ProviderID = "unselected"
				options = append(options, pkgsync.WithProvider("provider"))
			}
			runner := manualBindingRunner(t, builder, bindings, &failed, &resolutions, &calls)
			prepared, err := runner.Prepare(t.Context(), buildCatalog(t, builder), options...)
			if err != nil {
				t.Fatal(err)
			}
			want := int32(0)
			if selection == "filtered" {
				want = 1
			}
			if calls.Load() != want || resolutions.Load() != want {
				t.Fatalf("selection %s called %d/%d", selection, resolutions.Load(), calls.Load())
			}
			if len(prepared.Observations) != 1+int(want) {
				t.Fatal("unexpected provider observation")
			}
		})
	}
}

func TestStrictManualBindingsRequireExactSelection(t *testing.T) {
	builder, bindings := manualBindingFixture(t)
	var failed atomic.Bool
	var resolutions, calls atomic.Int32
	runner := manualBindingRunner(t, builder, bindings, &failed, &resolutions, &calls)
	inputs := catalogInputs{providerConfig: buildCatalog(t, builder)}
	configured, err := runner.bindProviderSources(createSourcesWithConfig(pkgsync.Defaults(), inputs, providerSourceComposition{}), pkgsync.Defaults(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	var bound []sources.Source
	for _, source := range configured {
		if source.ID() == sources.ProvidersID {
			bound = append(bound, source)
		}
	}
	observations := make([]sources.Observation, 0, 2)
	for _, binding := range bindings {
		observation, err := sources.NewObservation(sources.ProvidersID, buildCatalog(t, builder), manualBindingMetadata(&binding))
		if err != nil {
			t.Fatal(err)
		}
		observations = append(observations, observation)
	}
	if err := requireHealthyObservations(bound, observations); err != nil {
		t.Fatal(err)
	}
	for _, defect := range []string{"missing", "duplicate", "revision", "scope", "unscoped"} {
		t.Run(defect, func(t *testing.T) {
			changed := append([]sources.Observation(nil), observations...)
			switch defect {
			case "missing":
				changed = changed[:1]
			case "duplicate":
				changed[1] = changed[0]
			case "unscoped":
				changed[1].ProviderBinding = nil
			default:
				binding := *changed[1].ProviderBinding
				if defect == "revision" {
					binding.Revision = "2"
				} else {
					binding.AccountID = "another-account"
				}
				changed[1].ProviderBinding = &binding
			}
			if err := requireHealthyObservations(bound, changed); err == nil {
				t.Fatal("strict check accepted mismatched binding")
			}
		})
	}
}

func TestManualBindingVolumeUsesOnlyMatchingHistory(t *testing.T) {
	builder, bindings := manualBindingFixture(t)
	baseline := buildCatalog(t, builder)
	provenanceMap := provenance.Map{}
	for _, id := range []string{"one", "two"} {
		provenanceMap["model:"+provenance.ModelResourceID("provider", id)+":Name"] = []provenance.Entry{{Source: sources.ProvidersID, ProviderBindingID: id, ProviderBindingRevision: "1"}}
	}
	builder.SetProvenance(provenanceMap)
	baseline = buildCatalog(t, builder)
	provider, _ := builder.Provider("provider")
	delete(provider.Models, "two")
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, buildCatalog(t, builder), manualBindingMetadata(&bindings[0]))
	if err != nil {
		t.Fatal(err)
	}
	guarded, err := guardObservationVolume(baseline, observation)
	if err != nil || guarded.ID != observation.ID {
		t.Fatalf("peer history changed healthy receipt: %v", err)
	}
	binding := bindings[1]
	missing, err := sources.NewObservation(sources.ProvidersID, observation.Catalog, manualBindingMetadata(&binding))
	if err != nil {
		t.Fatal(err)
	}
	guarded, err = guardObservationVolume(baseline, missing)
	if err != nil {
		t.Fatal(err)
	}
	if guarded.ID == missing.ID || guarded.Status != sources.ObservationStatusDegraded || guarded.ProviderBinding == nil || *guarded.ProviderBinding != binding {
		t.Fatal("volume classification lost binding or failed to detect omission")
	}
	if err := guarded.Validate(); err != nil {
		t.Fatal(err)
	}
	binding.Revision = "2"
	updated, err := sources.NewObservation(sources.ProvidersID, observation.Catalog, manualBindingMetadata(&binding))
	if err != nil {
		t.Fatal(err)
	}
	guarded, err = guardObservationVolume(baseline, updated)
	if err != nil || guarded.ID != updated.ID {
		t.Fatal("old declaration was attributed to new revision")
	}
}

func TestManualBindingVolumeCannotBorrowAnotherProviderHistory(t *testing.T) {
	builder, bindings := manualBindingFixture(t)
	observation, err := sources.NewObservation(sources.ProvidersID, buildCatalog(t, builder), manualBindingMetadata(&bindings[0]))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := builder.Provider("provider")
	other.ID = "another-provider"
	other.Credentials = testcatalog.UnauthenticatedCredentials()
	if err := builder.SetProvider(other); err != nil {
		t.Fatal(err)
	}
	builder.SetProvenance(provenance.Map{
		"model:" + provenance.ModelResourceID("another-provider", "one") + ":Name": {{Source: sources.ProvidersID, ProviderBindingID: bindings[0].ID, ProviderBindingRevision: bindings[0].Revision}},
	})
	guarded, err := guardObservationVolume(buildCatalog(t, builder), observation)
	if err != nil {
		t.Fatal(err)
	}
	if guarded.ID != observation.ID {
		t.Fatal("another provider's history changed the bound receipt")
	}
}

func manualBindingMetadata(binding *sources.ProviderAcquisitionBinding) sources.ObservationMetadata {
	return sources.ObservationMetadata{ProviderBinding: binding, ObservedAt: time.Now().UTC(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Status: sources.ObservationStatusSucceeded, Completeness: sources.ObservationCompletenessComplete}
}

type blockedManualClient struct{ started chan<- struct{} }

func (c blockedManualClient) ListModels(ctx context.Context, _ sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	c.started <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestManualBindingsBoundConcurrencyAndCancelQueuedCalls(t *testing.T) {
	builder, original := manualBindingFixture(t)
	bindings := make([]sources.ProviderAcquisitionBinding, constants.MaxConcurrentProviders+3)
	for index := range bindings {
		bindings[index] = original[0]
		bindings[index].ID = "binding-" + strconv.Itoa(index)
	}
	started := make(chan struct{}, len(bindings))
	resolver := sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		return testcatalog.APIKeyMaterial(p.Credentials, "fixture"), nil
	})
	runner := NewBoundAcquisition(func(*catalogs.Provider) (sources.ProviderClient, error) {
		return blockedManualClient{started: started}, nil
	}, resolver, bindings)
	runner.loadEmbedded = func() (*catalogs.Builder, error) { return builder, nil }
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	baseline := buildCatalog(t, builder)
	go func() { _, err := runner.Prepare(ctx, baseline, pkgsync.WithSources(sources.ProvidersID)); done <- err }()
	for range constants.MaxConcurrentProviders {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("provider concurrency did not reach its configured limit")
		}
	}
	select {
	case <-started:
		t.Error("binding batch exceeded the provider concurrency limit")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !stderrors.Is(err, context.Canceled) {
			t.Fatalf("cancelled preparation = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued binding calls did not cancel")
	}
	if len(started) != 0 {
		t.Fatal("queued binding performed acquisition after cancellation")
	}
}
