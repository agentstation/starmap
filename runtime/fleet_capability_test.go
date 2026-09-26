package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type fleetCapabilityProbe struct {
	mu      sync.Mutex
	denied  bool
	request FleetAcquisitionRequirements
}

func (p *fleetCapabilityProbe) AcquireProviders(context.Context, AcquisitionRequest) (AcquisitionResult, error) {
	return AcquisitionResult{}, nil
}
func (p *fleetCapabilityProbe) CheckFleetAcquisition(_ context.Context, request FleetAcquisitionRequirements) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.request = request
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
