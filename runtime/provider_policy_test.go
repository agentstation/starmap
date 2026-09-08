package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderPolicyRestartExcludesInactiveEvidence(t *testing.T) {
	for _, selection := range []string{"removed", "new-revision", "unscoped", "missing-record"} {
		t.Run(selection, func(t *testing.T) {
			at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			layer := scopedProviderLayer(t, "binding", "1", at)
			old := layer
			if selection == "unscoped" {
				old = testProviderLayer(t, "provider", "model", "Model", at)
			}
			store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			stateDir := privateRuntimeDirectory(t)
			connected := openTestRuntime(t, WithStateDirectory(stateDir), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings(*layer.Receipt.ProviderBinding))
			if selection == "unscoped" {
				if err := connected.retainProviders(t.Context(), []ProviderLayer{old}); err != nil {
					t.Fatal(err)
				}
			} else if _, err := connected.publishProviders(t.Context(), []ProviderLayer{old}, connected.lease.epoch()); err != nil {
				t.Fatal(err)
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			if selection == "missing-record" {
				stateDir = privateRuntimeDirectory(t)
			}
			var active []sources.ProviderAcquisitionBinding
			if selection == "new-revision" {
				binding := *layer.Receipt.ProviderBinding
				binding.Revision = "2"
				active = append(active, binding)
			}
			reopened := openTestRuntime(t, WithStateDirectory(stateDir), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings(active...))
			if _, present := reopened.Catalog().Providers().Get("provider"); present {
				t.Error("inactive evidence returned in the effective catalog")
			}
			if selection != "missing-record" {
				retained, err := reopened.store.loadProviders()
				if err != nil {
					t.Fatal(err)
				}
				if len(retained) != 1 {
					t.Error("policy selection deleted retained evidence")
				}
			}
		})
	}
}

func TestProviderPolicyRejectsInactivePublicationBeforeWrites(t *testing.T) {
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	active := scopedProviderLayer(t, "binding", "2", at)
	for _, selection := range []string{"old-revision", "other-account", "unscoped", "undeclared"} {
		t.Run(selection, func(t *testing.T) {
			incoming := scopedProviderLayer(t, "binding", "1", at)
			switch selection {
			case "other-account":
				binding := *active.Receipt.ProviderBinding
				binding.AccountID = "different"
				incoming = providerLayerWithBinding(t, binding, at)
			case "unscoped":
				incoming = testProviderLayer(t, "provider", "model", "Model", at)
			case "undeclared":
				incoming = scopedProviderLayer(t, "different", "2", at)
			}
			connected := openTestRuntime(t, WithCatalogSource("embedded"), WithProviderBindings(*active.Receipt.ProviderBinding))
			before := connected.State()
			if _, err := connected.publishProviders(t.Context(), []ProviderLayer{active, incoming}, connected.lease.epoch()); err == nil {
				t.Error("inactive batch was accepted")
			}
			retained, err := connected.store.loadProviders()
			if err != nil {
				t.Fatal(err)
			}
			if len(retained) != 0 {
				t.Error("rejected batch wrote evidence")
			}
			if connected.State().Catalog != before.Catalog {
				t.Error("rejected batch changed effective catalog")
			}
		})
	}
}

func TestProviderPolicyRequiresNewRevisionForSelectorChanges(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding))
	if err := first.retainProviders(t.Context(), []ProviderLayer{layer}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	changed := *layer.Receipt.ProviderBinding
	changed.AccountID = "different"
	next, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithProviderBindings(changed))
	if err == nil {
		next.Close()
		t.Fatal("changed selectors reused a retained revision")
	}
}

func TestProviderPolicyOwnsDeclarations(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	bindings := []sources.ProviderAcquisitionBinding{*layer.Receipt.ProviderBinding}
	option := WithProviderBindings(bindings...)
	bindings[0].AccountID = "caller-change"
	connected := openTestRuntime(t, WithCatalogSource("embedded"), option)
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	if _, present := connected.Catalog().Providers().Get("provider"); !present {
		t.Fatal("active evidence is absent")
	}
}

func TestProviderPolicyRejectsDuplicateActiveIdentities(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	first := *layer.Receipt.ProviderBinding
	second := first
	second.Revision = "2"
	for _, bindings := range [][]sources.ProviderAcquisitionBinding{{first, first}, {first, second}, {{}}} {
		if _, err := defaults().apply(WithProviderBindings(bindings...)); err == nil {
			t.Error("invalid active set was accepted")
		}
	}
}

func TestProviderPolicyInactiveEvidenceCannotSatisfyStartupFreshness(t *testing.T) {
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	old := scopedProviderLayer(t, "binding", "1", at)
	current := scopedProviderLayer(t, "binding", "2", at)
	config, err := defaults().apply(WithProviderBindings(*current.Receipt.ProviderBinding))
	if err != nil {
		t.Fatal(err)
	}
	config.now = func() time.Time { return at }
	connected := providerOrderRuntime(t, false)
	connected.config = *config
	connected.layers.providerBindings = config.providerBindings
	connected.layers.setProvider(old)
	if !connected.acquisitionNeedsStartupPass() {
		t.Error("an inactive revision satisfied current acquisition freshness")
	}
	connected.layers.setProvider(current)
	if connected.acquisitionNeedsStartupPass() {
		t.Error("current fresh evidence requires another startup acquisition")
	}
}

func TestProviderPolicyWithoutDeclarationRejectsScopedEvidence(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	connected := openTestRuntime(t, WithCatalogSource("embedded"))
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err == nil {
		t.Error("scoped evidence activated without a declaration")
	}
	retained, err := connected.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 0 {
		t.Error("undeclared evidence reached durable retention")
	}
}

func TestProviderPolicyRefusalReportsEarlierPublication(t *testing.T) {
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	active := scopedProviderLayer(t, "binding", "1", at)
	inactive := scopedProviderLayer(t, "binding", "2", at)
	observer := windowAcquirerFunc(func(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
		if err := request.Publish(ctx, []ProviderLayer{active}); err != nil {
			return AcquisitionResult{}, err
		}
		return AcquisitionResult{Layers: []ProviderLayer{active, inactive}}, nil
	})
	connected := openTestRuntime(t, WithSource(newStubSource("idle")), WithAcquirer(observer), WithProviderBindings(*active.Receipt.ProviderBinding))
	report, err := connected.Refresh(t.Context())
	if err == nil {
		t.Fatal("inactive final evidence was accepted")
	}
	if !report.Published || report.GenerationID != connected.State().GenerationID {
		t.Error("policy refusal hid the earlier successful publication")
	}
	if !report.Acquisition.Published {
		t.Error("acquisition report lost the earlier publication")
	}
}

func TestProviderPolicyRefusesUnscopedAcquirerBeforeIO(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	observer := &stubAcquirer{}
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding), WithAcquirer(observer))
	if _, err := connected.Sync(t.Context()); err == nil {
		t.Error("unscoped acquisition role was accepted")
	}
	if observer.callCount() != 0 {
		t.Error("policy refusal happened after unscoped acquisition")
	}
}

func TestProviderPolicyEmptySetSkipsAcquisition(t *testing.T) {
	observer := &stubAcquirer{}
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithProviderBindings(), WithAcquirer(observer))
	if _, err := connected.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	if observer.callCount() != 0 {
		t.Error("empty active set invoked acquisition")
	}
}
