package providers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type bindingObserver interface {
	ObserveBinding(context.Context, sources.ProviderAcquisitionBinding) (sources.Observation, []sources.ProviderAttempt, error)
}

func observeTestBinding(t *testing.T, source *Source, binding sources.ProviderAcquisitionBinding) (sources.Observation, []sources.ProviderAttempt, error) {
	t.Helper()
	observer, ok := any(source).(bindingObserver)
	if !ok {
		t.Fatal("provider source cannot observe a declared acquisition binding")
	}
	return observer.ObserveBinding(t.Context(), binding)
}

func testAcquisitionBinding() sources.ProviderAcquisitionBinding {
	return sources.ProviderAcquisitionBinding{
		SchemaVersion: 1, ID: "deployment-binding", Revision: "1", ProviderID: "bound-provider",
		Public: true, Region: "global", APISurface: "models", CredentialRole: sources.ProviderBindingCatalogAcquisition,
		CredentialProfileID: "unauthenticated",
	}
}

func TestObserveBindingSelectsProviderAndRetainsReceipt(t *testing.T) {
	var resolutions, requests atomic.Int64
	delivered := make(chan sources.ProviderAttempt, 1)
	provider := providerForTest("bound-provider")
	source := newTestSource(newProviderSet(provider, providerForTest("other-provider")),
		WithAttemptSink(func(attempt sources.ProviderAttempt) { delivered <- attempt }),
		WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
			resolutions.Add(1)
			if p.ID != provider.ID || len(p.Credentials.CatalogAcquisition.Alternatives) != 1 || p.Credentials.CatalogAcquisition.Alternatives[0] != "unauthenticated" {
				t.Errorf("unexpected credential selection: %s", p.ID)
			}
			return sources.NewProviderCredentialMaterial(p.Credentials.Profiles[0], nil, sources.ProviderCredentialMetadata{}), nil
		})),
		WithClientFactory(func(p *catalogs.Provider) (sources.ProviderClient, error) {
			if p.ID != provider.ID {
				t.Errorf("requested an unselected provider")
			}
			return credentialRunClient(func(_ context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
				requests.Add(1)
				if material.Profile().ID != "unauthenticated" {
					t.Errorf("wrong request profile")
				}
				return []catalogs.Model{{ID: "model", Name: "Model"}}, nil
			}), nil
		}),
	)
	binding := testAcquisitionBinding()
	observation, attempts, err := observeTestBinding(t, source, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != sources.ProviderOutcomeSucceeded || !attempts[0].Requested {
		t.Fatalf("attempts = %+v", attempts)
	}
	if attempts[0].BindingID != binding.ID || attempts[0].BindingRevision != binding.Revision {
		t.Fatal("returned attempt lost its binding")
	}
	select {
	case attempt := <-delivered:
		if attempt.BindingID != binding.ID || attempt.BindingRevision != binding.Revision {
			t.Fatal("attempt sink lost its binding")
		}
	default:
		t.Fatal("attempt sink did not receive the result")
	}
	if observation.ProviderBinding == nil || *observation.ProviderBinding != binding {
		t.Fatal("observation lost its declared binding")
	}
	if observation.Catalog.Providers().Len() != 1 || requests.Load() != 1 || resolutions.Load() != 1 {
		t.Fatalf("providers=%d requests=%d resolutions=%d", observation.Catalog.Providers().Len(), requests.Load(), resolutions.Load())
	}
	binding.Revision = "changed"
	if observation.ProviderBinding.Revision != "1" {
		t.Fatal("binding aliases caller state")
	}
	receipt, err := observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ProviderBinding == nil || *receipt.ProviderBinding != *observation.ProviderBinding {
		t.Fatal("receipt lost binding")
	}
}

func TestObserveBindingRejectsConfigurationBeforeResolution(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sources.ProviderAcquisitionBinding, *catalogs.Provider)
	}{
		{"invalid schema", func(b *sources.ProviderAcquisitionBinding, _ *catalogs.Provider) {
			b.SchemaVersion = sources.ProviderAcquisitionBindingSchemaVersion + 1
		}},
		{"unknown provider", func(b *sources.ProviderAcquisitionBinding, _ *catalogs.Provider) { b.ProviderID = "unknown" }},
		{"missing endpoint", func(_ *sources.ProviderAcquisitionBinding, p *catalogs.Provider) { p.Catalog = nil }},
		{"missing credentials", func(_ *sources.ProviderAcquisitionBinding, p *catalogs.Provider) { p.Credentials = nil }},
		{"undeclared profile", func(b *sources.ProviderAcquisitionBinding, _ *catalogs.Provider) { b.CredentialProfileID = "unknown" }},
		{"inference only", func(_ *sources.ProviderAcquisitionBinding, p *catalogs.Provider) {
			p.Credentials.CatalogAcquisition.Alternatives = nil
		}},
		{"missing profile definition", func(_ *sources.ProviderAcquisitionBinding, p *catalogs.Provider) { p.Credentials.Profiles = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding, provider := testAcquisitionBinding(), providerForTest("bound-provider")
			tt.mutate(&binding, &provider)
			var resolutions, clients atomic.Int64
			source := newTestSource(newProviderSet(provider), WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
				resolutions.Add(1)
				return sources.ProviderCredentialMaterial{}, nil
			})), WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
				clients.Add(1)
				return receiptlessBindingClient{}, nil
			}))
			_, _, err := observeTestBinding(t, source, binding)
			if err == nil {
				t.Fatal("invalid binding reached acquisition")
			}
			var validation *pkgerrors.ValidationError
			var missing *pkgerrors.NotFoundError
			if !errors.As(err, &validation) && !errors.As(err, &missing) {
				t.Fatalf("untyped configuration error: %T", err)
			}
			if resolutions.Load() != 0 || clients.Load() != 0 {
				t.Fatalf("resolutions=%d clients=%d", resolutions.Load(), clients.Load())
			}
		})
	}
}

type receiptlessBindingClient struct{}

func (receiptlessBindingClient) ListModels(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	return nil, nil
}

func TestObserveBindingRefusesResolverProfileMismatch(t *testing.T) {
	var clients atomic.Int64
	source := newTestSource(newProviderSet(providerForTest("bound-provider")), WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		return sources.NewProviderCredentialMaterial(catalogs.ProviderCredentialProfile{ID: "different", Primitive: catalogs.ProviderAuthenticationNone}, nil, sources.ProviderCredentialMetadata{}), nil
	})), WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		clients.Add(1)
		return receiptlessBindingClient{}, nil
	}))
	observation, attempts, err := observeTestBinding(t, source, testAcquisitionBinding())
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Requested || observation.Status != sources.ObservationStatusDegraded || clients.Load() != 0 {
		t.Fatalf("mismatched profile sent a request: %+v", attempts)
	}
	if err := observation.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestObserveBindingConcurrentProfileSelection(t *testing.T) {
	provider := providerForTest("bound-provider")
	t.Setenv("STARMAP_BOUND_PROVIDER_API_KEY", "binding-test-credential")
	provider.Credentials = requiredAPIKeyCredentials("STARMAP_CSP3_BINDING_TEST_API_KEY")
	firstProfile := provider.Credentials.Profiles[0]
	firstProfile.ID = "first"
	secondProfile := firstProfile
	secondProfile.ID = "second"
	provider.Credentials.Profiles = []catalogs.ProviderCredentialProfile{firstProfile, secondProfile}
	provider.Credentials.CatalogAcquisition.Alternatives = []catalogs.ProviderCredentialProfileID{"first", "second"}
	provider.Credentials.Inference.Alternatives = []catalogs.ProviderCredentialProfileID{"first"}
	firstReady, releaseFirst := make(chan struct{}), make(chan struct{})
	defer close(releaseFirst)
	var clients atomic.Int64
	profiles := make(chan catalogs.ProviderCredentialProfileID, 2)
	source := newTestSource(newProviderSet(provider), WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		if clients.Add(1) == 1 {
			close(firstReady)
			select {
			case <-releaseFirst:
			case <-t.Context().Done():
				return nil, t.Context().Err()
			}
		}
		return credentialRunClient(func(_ context.Context, m sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
			profiles <- m.Profile().ID
			return []catalogs.Model{{ID: "model", Name: "Model"}}, nil
		}), nil
	}))
	observer, ok := any(source).(bindingObserver)
	if !ok {
		t.Fatal("provider source cannot observe bindings")
	}
	first := testAcquisitionBinding()
	first.CredentialProfileID = "first"
	second := first
	second.ID = "second-binding"
	second.CredentialProfileID = "second"
	type result struct {
		observation sources.Observation
		err         error
	}
	firstResult := make(chan result, 1)
	go func() { o, _, err := observer.ObserveBinding(t.Context(), first); firstResult <- result{o, err} }()
	select {
	case <-firstReady:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	observed, attempts, err := observer.ObserveBinding(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != sources.ProviderOutcomeSucceeded {
		t.Fatalf("second attempt = %+v", attempts)
	}
	if observed.ProviderBinding == nil || *observed.ProviderBinding != second {
		t.Fatal("second receipt changed binding")
	}
	if got := <-profiles; got != "second" {
		t.Fatalf("selected default profile instead of bound profile: %s", got)
	}
	releaseFirst <- struct{}{}
	select {
	case firstObserved := <-firstResult:
		if firstObserved.err != nil {
			t.Fatal(firstObserved.err)
		}
		if firstObserved.observation.ProviderBinding == nil || *firstObserved.observation.ProviderBinding != first {
			t.Fatal("first receipt changed binding")
		}
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	if got := <-profiles; got != "first" {
		t.Fatalf("first request changed profile: %s", got)
	}
	original, _ := source.providers.Get(provider.ID)
	if len(original.Credentials.CatalogAcquisition.Alternatives) != 2 {
		t.Fatal("binding changed source credential alternatives")
	}
}

func TestObserveBindingCanceledBeforeResolution(t *testing.T) {
	source := newTestSource(newProviderSet(providerForTest("bound-provider")), WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		t.Error("canceled binding resolved credentials")
		return sources.ProviderCredentialMaterial{}, nil
	})))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err := source.ObserveBinding(ctx, testAcquisitionBinding())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
