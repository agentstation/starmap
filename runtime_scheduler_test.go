package starmap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/fleet"
)

// TestStartupSpreadDefaultsToFifteenMinutes proves the fleet startup spread.
// Every instance waits inside a fifteen-minute window before its first run, so
// a fleet restart never sends one burst of requests upstream.
func TestStartupSpreadDefaultsToFifteenMinutes(t *testing.T) {
	t.Parallel()

	const want = 15 * time.Minute
	if fleet.DefaultStartupSpread != want {
		t.Fatalf("fleet.DefaultStartupSpread = %s, want %s", fleet.DefaultStartupSpread, want)
	}
	if got := runtimeDefaults().startupSpread; got != want {
		t.Fatalf("default startup spread = %s, want %s", got, want)
	}

	runtime, err := Open(context.Background(),
		WithStateDirectory(t.TempDir()),
		WithCatalogSource("embedded"),
		WithSourcePollInterval(0),
		WithAcquisitionEnabled(false),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	offsets := map[string]time.Duration{
		controllerSource:      runtime.schedule.sourceOffset,
		controllerAcquisition: runtime.schedule.acquisitionOffset,
	}
	for controller, offset := range offsets {
		if offset < 0 || offset >= want {
			t.Errorf("%s startup offset = %s, want a value inside [0, %s)", controller, offset, want)
		}
	}
	if runtime.schedule.sourceOffset == runtime.schedule.acquisitionOffset {
		t.Error("the two controllers must not share one startup offset")
	}
}

// TestStablePhaseSurvivesRestart proves that one instance keeps its place
// inside the refresh interval across a restart. The phase rests on the durable
// instance seed, so a fleet keeps its spread after a deployment.
func TestStablePhaseSurvivesRestart(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	const interval = time.Hour

	open := func() (identity string, sourcePhase, acquisitionPhase time.Duration) {
		t.Helper()
		runtime, err := Open(context.Background(),
			WithStateDirectory(directory),
			WithCatalogSource("embedded"),
			WithListenAddress("127.0.0.1:8080"),
			WithStartupSpread(0),
			WithSourcePollInterval(interval),
			WithAcquisitionEnabled(true),
			WithAcquisitionInterval(interval),
		)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer func() {
			if err := runtime.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
		return runtime.schedule.identity.Instance,
			runtime.schedule.sourcePhase,
			runtime.schedule.acquisitionPhase
	}

	firstIdentity, firstSource, firstAcquisition := open()
	secondIdentity, secondSource, secondAcquisition := open()

	if firstIdentity == "" {
		t.Fatal("the runtime derived no instance identity")
	}
	if firstIdentity != secondIdentity {
		t.Errorf("instance identity moved across a restart: %q then %q", firstIdentity, secondIdentity)
	}
	if firstSource != secondSource {
		t.Errorf("source phase moved across a restart: %s then %s", firstSource, secondSource)
	}
	if firstAcquisition != secondAcquisition {
		t.Errorf("acquisition phase moved across a restart: %s then %s",
			firstAcquisition, secondAcquisition)
	}
	if firstSource < 0 || firstSource >= interval {
		t.Errorf("source phase = %s, want a value inside [0, %s)", firstSource, interval)
	}
	if firstSource == firstAcquisition {
		t.Error("the two controllers must not share one phase")
	}

	// The seed is the durable part of the identity.
	seed := filepath.Join(directory, layerDirectoryName, instanceSeedFileName)
	if _, err := os.Stat(seed); err != nil {
		t.Fatalf("the runtime retained no instance seed: %v", err)
	}
}

// TestSchedulerIdentityDivergesAcrossClonedState proves that a copied state
// directory does not clone one schedule onto two instances. The identity
// combines the durable seed with the host name and the listen address, so two
// instances of one image keep separate phases.
func TestSchedulerIdentityDivergesAcrossClonedState(t *testing.T) {
	t.Parallel()

	origin := t.TempDir()
	const interval = time.Hour

	open := func(directory, address string) (string, time.Duration) {
		t.Helper()
		runtime, err := Open(context.Background(),
			WithStateDirectory(directory),
			WithCatalogSource("embedded"),
			WithListenAddress(address),
			WithStartupSpread(0),
			WithSourcePollInterval(interval),
			WithAcquisitionEnabled(false),
		)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer func() {
			if err := runtime.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
		return runtime.schedule.identity.Instance, runtime.schedule.sourcePhase
	}

	firstIdentity, firstPhase := open(origin, "10.0.0.1:8080")

	// Clone the retained state, exactly as a copied volume or image does.
	clone := t.TempDir()
	cloneLayers := filepath.Join(clone, layerDirectoryName)
	if err := os.MkdirAll(cloneLayers, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	seed, err := os.ReadFile(filepath.Join(origin, layerDirectoryName, instanceSeedFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	target := filepath.Join(cloneLayers, instanceSeedFileName)
	if err := os.WriteFile(target, seed, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	secondIdentity, secondPhase := open(clone, "10.0.0.2:8080")

	if secondIdentity == firstIdentity {
		t.Fatalf("the cloned state produced one identity %q for two instances", firstIdentity)
	}
	if secondPhase == firstPhase {
		t.Errorf("the cloned state produced one phase %s for two instances", firstPhase)
	}

	// The same instance keeps its identity. Only the address moved above.
	repeatIdentity, repeatPhase := open(origin, "10.0.0.1:8080")
	if repeatIdentity != firstIdentity {
		t.Errorf("instance identity moved without a change: %q then %q", firstIdentity, repeatIdentity)
	}
	if repeatPhase != firstPhase {
		t.Errorf("phase moved without a change: %s then %s", firstPhase, repeatPhase)
	}
}
