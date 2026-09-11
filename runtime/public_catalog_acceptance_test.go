package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/attestation"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func openPublicFixtureRuntime(t *testing.T, fixture *publicCatalogFixture, state string) *Runtime {
	t.Helper()
	runtime, err := Open(t.Context(), WithSourceURL(fixture.server.URL), WithStateDirectory(state), WithAcquisitionEnabled(false), WithSourcePollInterval(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("close runtime: %v", err)
		}
	})
	return runtime
}

func TestPublicCatalogActivatesWithoutProviderCredentials(t *testing.T) {
	fixture := newPublicCatalogFixture(t)
	runtime := openPublicFixtureRuntime(t, fixture, privateRuntimeDirectory(t))
	before := runtime.Status()
	if !before.Usable || !before.Fallback {
		t.Fatalf("initial catalog = %+v", before)
	}
	if fixture.requestCount() != 0 {
		t.Fatal("disabled polling started a source read")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	report, err := runtime.RefreshSource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	after := runtime.Status()
	if !report.Published || !after.Usable || after.Fallback || after.GenerationID != fixture.document.GenerationID {
		t.Fatalf("accepted catalog = %+v, refresh = %+v", after, report)
	}
	if len(after.Providers) != 0 || len(after.AcceptedAcquisitionSources) != 0 {
		t.Fatal("public refresh acquired provider evidence")
	}
	if runtime.Catalog() == nil || len(runtime.Catalog().Providers().List()) == 0 {
		t.Fatal("public catalog has no providers")
	}
}

func TestPublicCatalogRejectsUpdatesAndRetainsAcceptedState(t *testing.T) {
	fixture := newPublicCatalogFixture(t)
	state := privateRuntimeDirectory(t)
	runtime := openPublicFixtureRuntime(t, fixture, state)
	if _, err := runtime.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	accepted := runtime.State()
	for _, fault := range []string{"channel signature", "archive signature", "checksum", "size", "replay"} {
		t.Run(fault, func(t *testing.T) {
			fixture.setFault(fault)
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			report, err := runtime.RefreshSource(ctx)
			if err == nil || report.Published {
				t.Fatalf("invalid update = %+v, error = %v", report, err)
			}
			t.Logf("rejection: %v", err)
			switch fault {
			case "channel signature", "archive signature":
				var trust *attestation.TrustError
				if !errors.As(err, &trust) {
					t.Fatalf("signature failure = %T, want trust rejection", err)
				}
			case "checksum", "size":
				var invalid *pkgerrors.ValidationError
				if !errors.As(err, &invalid) || !strings.Contains(err.Error(), fault) {
					t.Fatalf("%s failure = %v, want matching validation rejection", fault, err)
				}
			case "replay":
				var conflict *pkgerrors.ConflictError
				if !errors.As(err, &conflict) || conflict.Resource != "catalog channel document" {
					t.Fatalf("replay failure = %v, want channel conflict", err)
				}
			}
			if got := runtime.State(); got.GenerationID != accepted.GenerationID || got.PayloadChecksum != accepted.PayloadChecksum {
				t.Fatalf("invalid update replaced accepted state: %+v", got)
			}
			if !runtime.Status().Usable {
				t.Fatal("invalid update removed the usable catalog")
			}
		})
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	fixture.setFault("unavailable")
	beforeReopen := fixture.requestCount()
	reopened := openPublicFixtureRuntime(t, fixture, state)
	if got := reopened.State(); got.GenerationID != accepted.GenerationID || got.PayloadChecksum != accepted.PayloadChecksum {
		t.Fatalf("restart lost accepted state: %+v", got)
	}
	if !reopened.Status().Usable {
		t.Fatal("restart lost the usable catalog")
	}
	if fixture.requestCount() != beforeReopen {
		t.Fatal("retained startup attempted an unscheduled source read")
	}
}
