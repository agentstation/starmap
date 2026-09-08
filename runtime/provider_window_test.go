package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type windowAcquirerFunc func(context.Context, AcquisitionRequest) (AcquisitionResult, error)

func (f windowAcquirerFunc) AcquireProviders(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
	return f(ctx, request)
}

func (f windowAcquirerFunc) AcquireProviderBindings(ctx context.Context, request AcquisitionRequest, _ []sources.ProviderAcquisitionBinding) (AcquisitionResult, error) {
	return f(ctx, request)
}

func TestProviderWindowPreservesDistinctEvidence(t *testing.T) {
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"account", "revision", "unscoped", "newer", "repeated", "forged"} {
		t.Run(scenario, func(t *testing.T) {
			first := scopedProviderLayer(t, "account-a", "1", at)
			final := first
			count := 2
			rejected := scenario == "forged" || scenario == "revision" || scenario == "unscoped"
			switch scenario {
			case "account":
				final = scopedProviderLayer(t, "account-b", "1", at)
			case "revision":
				final = scopedProviderLayer(t, "account-a", "2", at)
			case "unscoped":
				final = testProviderLayer(t, "provider", "model", "Model", at)
			case "newer":
				final = scopedProviderLayer(t, "account-a", "1", at.Add(time.Minute))
				count = 1
			case "repeated":
				count = 1
			case "forged":
				final.Receipt = final.Receipt.Clone()
				final.Receipt.ProviderBinding.AccountID = "forged"
				count = 1
			}
			if rejected {
				count = 1
			}
			bindings := []sources.ProviderAcquisitionBinding{*first.Receipt.ProviderBinding}
			if scenario == "account" {
				bindings = append(bindings, *final.Receipt.ProviderBinding)
			}
			var connected *Runtime
			var afterWindow uint64
			acquirer := windowAcquirerFunc(func(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
				if err := request.Publish(ctx, []ProviderLayer{first}); err != nil {
					return AcquisitionResult{}, err
				}
				afterWindow = connected.State().Sequence
				return AcquisitionResult{Layers: []ProviderLayer{first, final}}, nil
			})
			connected = openTestRuntime(t, WithCatalogSource("embedded"), WithAcquirer(acquirer), WithProviderBindings(bindings...))
			_, err := connected.Sync(t.Context())
			if rejected {
				if err == nil {
					t.Error("invalid or inactive final evidence bypassed validation after a window publication")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			retained, err := connected.store.loadProviders()
			if err != nil {
				t.Fatal(err)
			}
			if len(retained) != count {
				t.Errorf("retained %d scopes, want %d", len(retained), count)
			}
			if !rejected && retained[final.evidenceKey()].Receipt.Link.ObservationID != final.Receipt.Link.ObservationID {
				t.Error("the final observation did not reach durable retention")
			}
			if scenario == "repeated" || rejected {
				if connected.State().Sequence != afterWindow {
					t.Error("duplicate or invalid result published another generation")
				}
			}
		})
	}
}

func TestProviderWindowReportsRetainedPeerScope(t *testing.T) {
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	peer := scopedProviderLayer(t, "account-b", "1", at)
	answered := scopedProviderLayer(t, "account-a", "1", at.Add(time.Minute))
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "partial-failure"}[failed], func(t *testing.T) {
			acquirer := windowAcquirerFunc(func(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
				if err := request.Publish(ctx, []ProviderLayer{answered}); err != nil {
					return AcquisitionResult{}, err
				}
				result := AcquisitionResult{Layers: []ProviderLayer{answered}}
				if failed {
					return result, errors.New("another scope failed")
				}
				return result, nil
			})
			connected := openTestRuntime(t, WithCatalogSource("embedded"), WithAcquirer(acquirer), WithProviderBindings(*peer.Receipt.ProviderBinding, *answered.Receipt.ProviderBinding))
			if err := connected.retainProviders(t.Context(), []ProviderLayer{peer}); err != nil {
				t.Fatal(err)
			}
			report, err := connected.Sync(t.Context())
			if (err != nil) != failed {
				t.Fatalf("Sync error = %v, want failure %t", err, failed)
			}
			if len(report.Retained) != 1 || report.Retained[0] != catalogs.ProviderID("provider") {
				t.Errorf("retained report = %v, want the provider whose peer scope did not answer", report.Retained)
			}
			retained, err := connected.store.loadProviders()
			if err != nil {
				t.Fatal(err)
			}
			if retained[peer.evidenceKey()].Receipt.Link.ObservationID != peer.Receipt.Link.ObservationID {
				t.Error("a successful peer replaced the unanswered scope")
			}
		})
	}
}
