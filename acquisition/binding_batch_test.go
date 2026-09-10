package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func boundBatchRole(t *testing.T, acquirer *Acquirer) runtime.BindingAcquirer {
	t.Helper()
	role, ok := any(acquirer).(runtime.BindingAcquirer)
	if !ok {
		t.Fatal("built-in acquirer cannot run the active binding set")
	}
	return role
}

func TestAcquireBindingsKeepsTwoScopesForOneProvider(t *testing.T) {
	current, first := acquisitionBindingFixture(t)
	first.ID, first.Public, first.AccountID = "first", false, "account-one"
	second := first
	second.ID, second.AccountID = "second", "account-two"
	observer := newProviderSourceObserver(nil)
	observer.options = append(observer.options, providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) { return receiptProviderClient{}, nil }))
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	result, err := boundBatchRole(t, acquirer).AcquireProviderBindings(context.Background(), runtime.AcquisitionRequest{Current: current}, []sources.ProviderAcquisitionBinding{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligible != 2 || len(result.Attempts) != 2 || len(result.Layers) != 2 {
		t.Fatalf("lost a binding: %+v", result)
	}
	found := map[string]sources.ProviderAcquisitionBinding{}
	for _, layer := range result.Layers {
		binding := layer.Receipt.ProviderBinding
		if binding == nil {
			t.Fatal("bound batch returned an unscoped receipt")
		}
		found[binding.ID] = *binding
	}
	if found[first.ID] != first || found[second.ID] != second {
		t.Fatal("batch collapsed distinct provider accounts")
	}
}

type batchObserverFunc func(context.Context, *catalogs.Catalog, sources.ProviderAcquisitionBinding) (ProviderObservation, error)

func (f batchObserverFunc) ObserveProviderBinding(ctx context.Context, current *catalogs.Catalog, binding sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
	return f(ctx, current, binding)
}

func (f batchObserverFunc) ObserveProvider(context.Context, *catalogs.Catalog, catalogs.ProviderID) (ProviderObservation, error) {
	return ProviderObservation{}, stderrors.New("unscoped observation is forbidden")
}

func batchFixture(t *testing.T) (*catalogs.Catalog, []sources.ProviderAcquisitionBinding, *providerSourceObserver) {
	t.Helper()
	current, first := acquisitionBindingFixture(t)
	first.ID, first.Public, first.AccountID = "first", false, "account-one"
	second := first
	second.ID, second.AccountID = "second", "account-two"
	observer := newProviderSourceObserver(nil)
	observer.options = append(observer.options, providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) { return receiptProviderClient{}, nil }))
	return current, []sources.ProviderAcquisitionBinding{first, second}, observer
}

func TestAcquireBindingsValidatesBatchBeforeIO(t *testing.T) {
	for _, defect := range []string{"schema", "duplicate", "duplicate-revision", "unknown-profile", "unknown-provider", "missing-current", "unbound-filter"} {
		t.Run(defect, func(t *testing.T) {
			current, bindings, observer := batchFixture(t)
			var resolutions, clients atomic.Int64
			observer.options = append(observer.options,
				providers.WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
					resolutions.Add(1)
					return sources.NewProviderCredentialMaterial(p.Credentials.Profiles[0], nil, sources.ProviderCredentialMetadata{}), nil
				})),
				providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
					clients.Add(1)
					return receiptProviderClient{}, nil
				}),
			)
			request := runtime.AcquisitionRequest{Current: current}
			switch defect {
			case "schema":
				bindings[1].SchemaVersion = sources.ProviderAcquisitionBindingSchemaVersion + 1
			case "duplicate":
				bindings[1] = bindings[0]
			case "duplicate-revision":
				bindings[1] = bindings[0]
				bindings[1].Revision = "2"
			case "unknown-profile":
				bindings[1].CredentialProfileID = "missing"
			case "unknown-provider":
				bindings[1].ProviderID = "missing"
			case "missing-current":
				request.Current = nil
			case "unbound-filter":
				request.Providers = []catalogs.ProviderID{"missing"}
			}
			acquirer, err := NewAcquirer(WithProviderObserver(observer))
			if err != nil {
				t.Fatal(err)
			}
			result, err := acquirer.AcquireProviderBindings(t.Context(), request, bindings)
			if err == nil {
				t.Fatal("invalid batch was accepted")
			}
			if result.Eligible != 0 || len(result.Attempts) != 0 || resolutions.Load() != 0 || clients.Load() != 0 {
				t.Fatal("invalid batch reached credential or client work")
			}
		})
	}
}

func TestAcquireBindingsEmptySetAndProviderFilter(t *testing.T) {
	current, bindings, fallback := batchFixture(t)
	var calls atomic.Int64
	observer := batchObserverFunc(func(ctx context.Context, current *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
		calls.Add(1)
		return fallback.ObserveProviderBinding(ctx, current, b)
	})
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	for _, empty := range [][]sources.ProviderAcquisitionBinding{nil, {}} {
		result, err := acquirer.AcquireProviderBindings(t.Context(), runtime.AcquisitionRequest{Current: current}, empty)
		if err != nil || result.Eligible != 0 || calls.Load() != 0 {
			t.Fatal("empty bindings selected ambient providers")
		}
	}
	bindings[1].ProviderID = "unselected"
	result, err := acquirer.AcquireProviderBindings(t.Context(), runtime.AcquisitionRequest{Current: current, Providers: []catalogs.ProviderID{bindings[0].ProviderID}}, bindings)
	if err != nil || result.Eligible != 1 || calls.Load() != 1 {
		t.Fatalf("filter result: %+v, %v", result, err)
	}
	unscoped, err := NewAcquirer(WithProviderObserver(unscopedOnlyObserver{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unscoped.AcquireProviderBindings(t.Context(), runtime.AcquisitionRequest{Current: current}, bindings[:1]); err == nil {
		t.Fatal("bound batch accepted an unscoped-only observer")
	}
}

func TestAcquireBindingsRecordsScopedFailureAndSkip(t *testing.T) {
	current, bindings, fallback := batchFixture(t)
	third := bindings[0]
	third.ID = "third"
	bindings = append(bindings, third)
	observer := batchObserverFunc(func(ctx context.Context, current *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
		switch b.ID {
		case "first":
			return fallback.ObserveProviderBinding(ctx, current, b)
		case "second":
			return ProviderObservation{}, stderrors.New("private provider error must stay absent")
		default:
			return ProviderObservation{Attempt: sources.ProviderAttempt{ProviderID: b.ProviderID, Outcome: sources.ProviderOutcomeSkippedNotConfigured, Reason: sources.ProviderReasonCredentialUnavailable}}, nil
		}
	})
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	result, err := acquirer.AcquireProviderBindings(t.Context(), runtime.AcquisitionRequest{Current: current}, bindings)
	if err != nil || result.Eligible != 3 || len(result.Attempts) != 3 || len(result.Layers) != 1 {
		t.Fatalf("partial result: %+v, %v", result, err)
	}
	outcomes := []sources.ProviderOutcome{sources.ProviderOutcomeSucceeded, sources.ProviderOutcomeFailed, sources.ProviderOutcomeSkippedNotConfigured}
	for i, attempt := range result.Attempts {
		if err := attempt.Validate(); err != nil {
			t.Fatal(err)
		}
		if attempt.BindingID != bindings[i].ID || attempt.BindingRevision != bindings[i].Revision || attempt.Outcome != outcomes[i] {
			t.Fatalf("wrong scoped attempt: %+v", attempt)
		}
		if attempt.StartedAt.IsZero() || attempt.CompletedAt.IsZero() {
			t.Fatal("attempt lacks timestamps")
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("private provider error")) {
		t.Fatal("provider error escaped safe reason classification")
	}
}

func TestAcquireBindingsWindowOwnsInputsAndPublishedCopies(t *testing.T) {
	current, bindings, fallback := batchFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	observer := batchObserverFunc(func(ctx context.Context, current *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
		if b.ID == "second" {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ProviderObservation{}, ctx.Err()
			}
		}
		return fallback.ObserveProviderBinding(ctx, current, b)
	})
	acquirer, err := NewAcquirer(WithProviderObserver(observer), WithAcquirerCoalesceTimer(func(time.Duration) <-chan time.Time { ready := make(chan time.Time); close(ready); return ready }))
	if err != nil {
		t.Fatal(err)
	}
	var published atomic.Int64
	firstWindow := make(chan struct{})
	request := runtime.AcquisitionRequest{Current: current, Publish: func(_ context.Context, layers []runtime.ProviderLayer) error {
		if len(layers) != 1 || layers[0].Receipt.ProviderBinding.ID != "first" {
			t.Error("window mixed the unfinished binding")
		}
		layers[0].Payload[0] = '!'
		layers[0].Receipt.ProviderBinding.Revision = "callback-change"
		published.Add(1)
		close(firstWindow)
		return nil
	}}
	type answer struct {
		result runtime.AcquisitionResult
		err    error
	}
	done := make(chan answer, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		result, err := acquirer.AcquireProviderBindings(ctx, request, bindings)
		done <- answer{result, err}
	}()
	waitBatchSignal(t, entered)
	waitBatchSignal(t, firstWindow)
	bindings[0].Revision, bindings[1].Revision = "caller-change", "caller-change"
	close(release)
	select {
	case answer := <-done:
		if answer.err != nil {
			t.Fatal(answer.err)
		}
		if len(answer.result.Layers) != 2 || published.Load() != 1 {
			t.Fatal("lost the late binding")
		}
		for _, layer := range answer.result.Layers {
			if layer.Receipt.ProviderBinding.Revision != "1" {
				t.Fatal("retained result aliases caller or callback state")
			}
			if _, err := catalogs.DecodeSourceObservationPayload(layer.Payload); err != nil {
				t.Fatal(err)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("bound batch did not finish")
	}
}

func TestAcquireBindingsCancellationRejectsLateResults(t *testing.T) {
	current, bindings, fallback := batchFixture(t)
	entered := make(chan struct{}, len(bindings))
	release := make(chan struct{})
	returned := make(chan struct{}, len(bindings))
	observer := batchObserverFunc(func(_ context.Context, current *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
		entered <- struct{}{}
		<-release
		result, err := fallback.ObserveProviderBinding(context.Background(), current, b)
		returned <- struct{}{}
		return result, err
	})
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	var published atomic.Int64
	request := runtime.AcquisitionRequest{Current: current, Publish: func(context.Context, []runtime.ProviderLayer) error { published.Add(1); return nil }}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type answer struct {
		result runtime.AcquisitionResult
		err    error
	}
	done := make(chan answer, 1)
	go func() {
		result, err := acquirer.AcquireProviderBindings(ctx, request, bindings)
		done <- answer{result, err}
	}()
	for range bindings {
		waitBatchSignal(t, entered)
	}
	cancel()
	select {
	case answer := <-done:
		if !stderrors.Is(answer.err, context.Canceled) || len(answer.result.Layers) != 0 || len(answer.result.Attempts) != 2 {
			t.Fatalf("canceled result: %+v, %v", answer.result, answer.err)
		}
		for i, attempt := range answer.result.Attempts {
			if attempt.BindingID != bindings[i].ID || attempt.Reason != sources.ProviderReasonRequestTimeout || attempt.Requested {
				t.Fatalf("wrong unfinished binding: %+v", attempt)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not close the run")
	}
	close(release)
	for range bindings {
		waitBatchSignal(t, returned)
	}
	if published.Load() != 0 {
		t.Fatal("late answer reached publication")
	}
}

func TestAcquireBindingsPublicationFailureCancelsPendingBinding(t *testing.T) {
	current, bindings, fallback := batchFixture(t)
	stopped := make(chan struct{})
	entered := make(chan struct{})
	observer := batchObserverFunc(func(ctx context.Context, current *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
		if b.ID == "second" {
			close(entered)
			<-ctx.Done()
			close(stopped)
			return ProviderObservation{}, ctx.Err()
		}
		select {
		case <-entered:
		case <-ctx.Done():
			return ProviderObservation{}, ctx.Err()
		}
		return fallback.ObserveProviderBinding(ctx, current, b)
	})
	acquirer, err := NewAcquirer(WithProviderObserver(observer), WithAcquirerCoalesceTimer(func(time.Duration) <-chan time.Time { ready := make(chan time.Time); close(ready); return ready }))
	if err != nil {
		t.Fatal(err)
	}
	refused := stderrors.New("publication refused")
	result, err := acquirer.AcquireProviderBindings(t.Context(), runtime.AcquisitionRequest{Current: current, Publish: func(context.Context, []runtime.ProviderLayer) error { return refused }}, bindings)
	if !stderrors.Is(err, refused) || len(result.Layers) != 1 || len(result.Attempts) != 2 {
		t.Fatalf("publication failure: %+v, %v", result, err)
	}
	waitBatchSignal(t, stopped)
}

func waitBatchSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("batch operation did not reach its barrier")
	}
}

type batchClientFunc func(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error)

func (f batchClientFunc) ListModels(ctx context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	return f(ctx, material)
}

func TestAcquireBindingsKeepsConcurrentCredentialProfilesSeparate(t *testing.T) {
	current, bindings, observer := batchFixture(t)
	provider, _ := current.Providers().Get(bindings[0].ProviderID)
	provider.Credentials.Fields = []catalogs.ProviderCredentialField{{ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret, Required: true}}
	provider.Credentials.Profiles[0].Primitive = catalogs.ProviderAuthenticationAPIKey
	provider.Credentials.Profiles[0].Fields = []catalogs.ProviderCredentialFieldID{"api-key"}
	provider.Credentials.Profiles[0].Placements = []catalogs.ProviderCredentialPlacement{{Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader, Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer}}
	provider.Credentials.CatalogAcquisition.Required = true
	provider.Credentials.Inference.Required = true
	secondProfile := provider.Credentials.Profiles[0]
	secondProfile.ID = "second-profile"
	provider.Credentials.Profiles = append(provider.Credentials.Profiles, secondProfile)
	provider.Credentials.CatalogAcquisition.Alternatives = append(provider.Credentials.CatalogAcquisition.Alternatives, secondProfile.ID)
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	current, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	bindings[1].CredentialProfileID = secondProfile.ID
	arrived := make(chan catalogs.ProviderCredentialProfileID, 2)
	release := make(chan struct{})
	requested := make(chan catalogs.ProviderCredentialProfileID, 2)
	observer.options = append(observer.options,
		providers.WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(ctx context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
			if len(p.Credentials.CatalogAcquisition.Alternatives) != 1 {
				return sources.ProviderCredentialMaterial{}, stderrors.New("ambiguous profile selection")
			}
			id := p.Credentials.CatalogAcquisition.Alternatives[0]
			arrived <- id
			select {
			case <-release:
			case <-ctx.Done():
				return sources.ProviderCredentialMaterial{}, ctx.Err()
			}
			for _, profile := range p.Credentials.Profiles {
				if profile.ID == id {
					return sources.NewProviderCredentialMaterial(profile, map[catalogs.ProviderCredentialFieldID]string{"api-key": "fixture-" + string(id)}, sources.ProviderCredentialMetadata{}), nil
				}
			}
			return sources.ProviderCredentialMaterial{}, stderrors.New("missing selected profile")
		})),
		providers.WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
			return batchClientFunc(func(_ context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
				value, found := material.Value("api-key")
				if !found || value != "fixture-"+string(material.Profile().ID) {
					t.Error("provider received another profile's material")
				}
				requested <- material.Profile().ID
				return []catalogs.Model{{ID: string(material.Profile().ID), Name: "Observed Model"}}, nil
			}), nil
		}),
	)
	acquirer, err := NewAcquirer(WithProviderObserver(observer))
	if err != nil {
		t.Fatal(err)
	}
	type answer struct {
		result runtime.AcquisitionResult
		err    error
	}
	done := make(chan answer, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		result, err := acquirer.AcquireProviderBindings(ctx, runtime.AcquisitionRequest{Current: current}, bindings)
		done <- answer{result, err}
	}()
	resolved := map[catalogs.ProviderCredentialProfileID]int{}
	for range bindings {
		select {
		case id := <-arrived:
			resolved[id]++
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent profile selection did not finish")
		}
	}
	close(release)
	select {
	case answer := <-done:
		if answer.err != nil || len(answer.result.Layers) != 2 {
			t.Fatalf("profile run: %+v, %v", answer.result, answer.err)
		}
		for i, layer := range answer.result.Layers {
			if layer.Receipt.ProviderBinding.CredentialProfileID != bindings[i].CredentialProfileID {
				t.Fatal("receipt lost selected profile")
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("profile run did not finish")
	}
	sent := map[catalogs.ProviderCredentialProfileID]int{}
	for range bindings {
		select {
		case id := <-requested:
			sent[id]++
		default:
			t.Fatal("provider did not receive selected material")
		}
	}
	for _, b := range bindings {
		if resolved[b.CredentialProfileID] != 1 || sent[b.CredentialProfileID] != 1 {
			t.Fatal("concurrent scopes shared a credential selection")
		}
	}
}

func TestAcquireBindingsRejectsMismatchedObserverResults(t *testing.T) {
	for _, defect := range []string{"receipt", "attempt-provider", "attempt-binding", "attempt-revision", "layer-provider", "missing-payload", "failed-with-payload", "invalid-outcome"} {
		t.Run(defect, func(t *testing.T) {
			current, bindings, fallback := batchFixture(t)
			observer := batchObserverFunc(func(ctx context.Context, current *catalogs.Catalog, b sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
				result, err := fallback.ObserveProviderBinding(ctx, current, b)
				if err != nil {
					return result, err
				}
				switch defect {
				case "receipt":
					result.Layer.Receipt.ProviderBinding.AccountID = "different"
				case "attempt-provider":
					result.Attempt.ProviderID = "different"
				case "attempt-binding":
					result.Attempt.BindingID = "different"
				case "attempt-revision":
					result.Attempt.BindingRevision = "different"
				case "layer-provider":
					result.Layer.ProviderID = "different"
				case "missing-payload":
					result.Layer = runtime.ProviderLayer{}
				case "failed-with-payload":
					result.Attempt.Outcome = sources.ProviderOutcomeFailed
					result.Attempt.Reason = sources.ProviderReasonTransportFailed
				case "invalid-outcome":
					result.Attempt.Outcome = "unknown"
				}
				return result, nil
			})
			acquirer, err := NewAcquirer(WithProviderObserver(observer), WithAcquirerCoalesceTimer(func(time.Duration) <-chan time.Time {
				t.Error("invalid observation opened a publication window")
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			result, err := acquirer.AcquireProviderBindings(t.Context(), runtime.AcquisitionRequest{Current: current}, bindings[:1])
			if err != nil || len(result.Layers) != 0 || len(result.Attempts) != 1 || result.Attempts[0].Outcome != sources.ProviderOutcomeFailed {
				t.Fatalf("invalid observation escaped: %+v, %v", result, err)
			}
		})
	}
}
