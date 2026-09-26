package runtime

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type fleetCapabilityProbe struct {
	mu                      sync.Mutex
	denied                  bool
	request                 FleetAcquisitionRequirements
	requireProviderMetadata bool
}

func (p *fleetCapabilityProbe) AcquireProviders(context.Context, AcquisitionRequest) (AcquisitionResult, error) {
	return AcquisitionResult{}, nil
}
func (p *fleetCapabilityProbe) CheckFleetAcquisition(_ context.Context, request FleetAcquisitionRequirements) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.request = request
	if p.requireProviderMetadata {
		for _, id := range request.Providers {
			if _, err := request.Catalog.Provider(id); err != nil {
				return err
			}
		}
	}
	if p.denied {
		return errors.New("private credential location must not escape")
	}
	return nil
}

func TestFleetCapabilityRefusesOwnershipAndRecovers(t *testing.T) {
	probe := &fleetCapabilityProbe{denied: true}
	backend := newFleetRuntimeBackend(t, WithAcquirer(probe))
	connected := openFleetRuntime(t, backend, "capability", privateRuntimeDirectory(t), WithAcquirer(probe))
	// Retained provider inputs require acquisition access before the next ownership grant.
	connected.mu.Lock()
	connected.layers.providers = map[providerEvidenceKey]ProviderLayer{}
	connected.layers.setProvider(ProviderLayer{ProviderID: catalogs.ProviderID("required")})
	connected.mu.Unlock()
	old := connected.lease.epoch()
	if err := connected.lease.renewOnce(t.Context()); err == nil {
		t.Fatal("renewed without required acquisition access")
	}
	if connected.lease.status() != leaseLost {
		t.Fatal("kept ownership after access loss")
	}
	backend.mu.Lock()
	held := backend.lease
	backend.mu.Unlock()
	if held.Holder != "" || !held.ExpiresAt.IsZero() {
		t.Fatal("did not return the unavailable owner's grant")
	}
	if err := connected.lease.ensureHeld(t.Context()); err == nil {
		t.Fatal("acquired before capability recovery")
	}
	status, _ := connected.FleetStatus()
	if status.AcquisitionReady {
		t.Fatal("reported unavailable capability as ready")
	}
	probe.mu.Lock()
	probe.denied = false
	probe.mu.Unlock()
	if err := connected.lease.ensureHeld(t.Context()); err != nil {
		t.Fatal(err)
	}
	if connected.lease.epoch() <= old {
		t.Fatal("recovery reused an ended grant")
	}
	probe.mu.Lock()
	providers := probe.request.Providers
	probe.mu.Unlock()
	if len(providers) != 1 || providers[0] != "required" {
		t.Fatalf("required providers: %v", providers)
	}
	status, _ = connected.FleetStatus()
	if !status.AcquisitionReady {
		t.Fatal("capability recovery not reported")
	}
}

func TestFleetCapabilityChecksActiveBindings(t *testing.T) {
	probe := &fleetCapabilityProbe{}
	connected := openFleetRuntime(t, newFleetRuntimeBackend(t, WithAcquirer(probe)), "bindings", privateRuntimeDirectory(t), WithAcquirer(probe))
	binding := sources.ProviderAcquisitionBinding{ID: "required-binding", ProviderID: "required"}
	connected.mu.Lock()
	connected.layers.providerBindings = &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{binding.ID: binding}}
	connected.mu.Unlock()
	if err := connected.checkFleetAcquisition(t.Context()); err != nil {
		t.Fatal(err)
	}
	probe.mu.Lock()
	got := probe.request.Bindings
	probe.mu.Unlock()
	if len(got) != 1 || got[0] != binding {
		t.Fatalf("required bindings: %v", got)
	}
	missing := &Runtime{client: connected.client, layers: connected.layers}
	if err := missing.checkFleetAcquisition(t.Context()); err == nil {
		t.Fatal("accepted a missing capability checker")
	}
}

func TestFleetCapabilityChecksUnobservedProvidersBeforeGrant(t *testing.T) {
	probe := &fleetCapabilityProbe{denied: true}
	backend := newFleetRuntimeBackend(t, WithAcquirer(probe))
	backend.head = FleetHead{}
	connected := openFleetRuntime(t, backend, "unobserved", privateRuntimeDirectory(t), WithAcquirer(probe))
	if len(connected.layers.providers) != 0 {
		t.Fatal("fixture already has retained provider observations")
	}
	providers := connected.Catalog().Providers().List()
	if len(providers) == 0 {
		t.Fatal("fixture has no eligible providers")
	}
	if connected.lease.status() != leaseLost {
		t.Fatal("replica took ownership without access to an unobserved provider")
	}
	backend.mu.Lock()
	attempts := backend.acquisitions
	backend.mu.Unlock()
	if attempts != 0 {
		t.Fatal("capability refusal reached lease acquisition")
	}
	probe.mu.Lock()
	for _, provider := range providers {
		if !slices.Contains(probe.request.Providers, provider.ID) {
			t.Errorf("unobserved provider %q is missing from the capability check", provider.ID)
		}
	}
	probe.denied = false
	probe.mu.Unlock()
	if err := connected.lease.ensureHeld(t.Context()); err != nil {
		t.Fatal(err)
	}
	if connected.lease.status() != leaseHeld {
		t.Fatal("replica did not recover ownership after capability recovery")
	}
}

func TestFleetCapabilityRetainsProviderMetadataAcrossPin(t *testing.T) {
	probe := &fleetCapabilityProbe{requireProviderMetadata: true}
	backend := newFleetRuntimeBackend(t, WithAcquirer(probe))
	open := func(id, pin string) *Runtime {
		return openFleetRuntime(t, backend, id, privateRuntimeDirectory(t), WithAcquirer(probe), WithGenerationPin(pin))
	}
	leader := open("metadata-leader", "")
	selected := leader.State().GenerationID
	layer := testProviderLayer(t, "later-provider", "model", "Later Model", time.Now().UTC())
	source := leader.source.(*stubSource)
	source.mu.Lock()
	source.replies = []SourceRead{testSourceRead(t, "later-source", layer.Payload, time.Now().UTC())}
	source.mu.Unlock()
	if _, err := leader.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, err := leader.execute(t.Context(), runKindAcquisition, func(ctx context.Context, _ *RefreshReport, epoch uint64) error {
		_, err := leader.publishInputChanges(ctx, nil, []ProviderLayer{layer}, epoch)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := leader.Close(); err != nil {
		t.Fatal(err)
	}
	pinned := open("metadata-pin-owner", selected)
	if _, err := pinned.Catalog().Provider(layer.ProviderID); err == nil {
		t.Fatal("pin served a provider from a later generation")
	}
	if err := pinned.lease.renewOnce(t.Context()); err != nil {
		t.Fatal("pin lost ownership while retained acquisition access remained available:", err)
	}
	probe.mu.Lock()
	metadata, err := probe.request.Catalog.Provider(layer.ProviderID)
	probe.mu.Unlock()
	if err != nil || len(metadata.Models) != 0 {
		t.Fatal("capability metadata retained model payloads:", err)
	}
	if err := pinned.Close(); err != nil {
		t.Fatal(err)
	}
	unpin := open("metadata-unpin-owner", "")
	status, _ := unpin.FleetStatus()
	if !status.AcquisitionReady || unpin.lease.status() != leaseHeld {
		t.Fatal("unpin could not acquire refresh ownership")
	}
	provider, err := unpin.Catalog().Provider(layer.ProviderID)
	if err != nil || provider.Models["model"] == nil {
		t.Fatal("unpin lost the retained provider model:", err)
	}
}
