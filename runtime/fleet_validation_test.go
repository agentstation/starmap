package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fleetValidationFixture(t *testing.T) (FleetSnapshot, layerSet) {
	t.Helper()
	snapshot, layers := fleetReplayFixture(t)
	var err error
	layers.sourceConfiguration, err = describeSources(defaults())
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data, err = encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(snapshot.Publication.Recovery.Data)
	snapshot.Head = snapshot.Publication.nextHead()
	return snapshot, layers
}

func TestValidateFleetReplayOffline(t *testing.T) {
	snapshot, layers := fleetValidationFixture(t)
	source := newStubSource("retained-source")
	generation := snapshot.Publication.Generation
	layers.source = &sourceLayer{Identity: source.Identity(), GenerationID: generation.Manifest.GenerationID,
		Checksum: generation.Manifest.Payload.Checksum, Payload: generation.Payload, Manifest: &generation.Manifest,
		PublishedAt: generation.Manifest.GeneratedAt}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data = data
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(data)
	snapshot.Head = snapshot.Publication.nextHead()
	before := snapshot
	directory := filepath.Join(t.TempDir(), "must-not-be-created")
	if err := ValidateFleetRecovery(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFleetReplay(t.Context(), snapshot, WithSource(source), WithStateDirectory(directory)); err != nil {
		t.Fatal(err)
	}
	if source.reads != 0 {
		t.Fatal("validation read its external source")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("validation created state: %v", err)
	}
	if !reflect.DeepEqual(snapshot, before) {
		t.Fatal("validation changed publication evidence")
	}
	if err := ValidateFleetReplay(t.Context(), snapshot, WithSource(newStubSource("different"))); err == nil {
		t.Fatal("accepted a different source")
	}
	if err := ValidateFleetReplay(t.Context(), snapshot, WithSource(source), WithSourceAliases("changed")); err == nil {
		t.Fatal("accepted a different acquisition policy")
	}
}

func TestValidateFleetRecoveryRejectsCorruption(t *testing.T) {
	for _, name := range []string{"checksum", "schema", "generation"} {
		t.Run(name, func(t *testing.T) {
			snapshot, _ := fleetValidationFixture(t)
			switch name {
			case "checksum":
				snapshot.Publication.Recovery.Data = []byte("corrupt")
			case "schema":
				snapshot.Publication.Recovery.Data = []byte(`{"version":999}`)
				snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(snapshot.Publication.Recovery.Data)
				snapshot.Head = snapshot.Publication.nextHead()
			case "generation":
				snapshot.Head.GenerationID = "different"
			}
			if ValidateFleetRecovery(t.Context(), snapshot) == nil {
				t.Fatal("accepted corrupt retained inputs")
			}
			if ValidateFleetReplay(t.Context(), snapshot) == nil {
				t.Fatal("replayed corrupt retained inputs")
			}
		})
	}
}

func TestValidateFleetRecoveryContext(t *testing.T) {
	snapshot, _ := fleetValidationFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for name, validate := range map[string]func(context.Context, FleetSnapshot) error{
		"recovery": ValidateFleetRecovery,
		"replay":   func(ctx context.Context, snapshot FleetSnapshot) error { return ValidateFleetReplay(ctx, snapshot) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := validate(nil, snapshot); err == nil {
				t.Fatal("accepted a missing context")
			}
			if err := validate(ctx, snapshot); !errors.Is(err, context.Canceled) {
				t.Fatalf("ignored cancellation: %v", err)
			}
		})
	}
}

func TestValidateFleetReplayPreservesPin(t *testing.T) {
	snapshot, layers := fleetValidationFixture(t)
	manifest := snapshot.Publication.Generation.Manifest
	config := defaults()
	config.resolve()
	probe := &Runtime{config: *config}
	pin := generationPinRecord{Version: generationPinRecordVersion, Binding: probe.pinBinding(), Phase: pinAccepted,
		Receipt: GenerationPinAcceptance{OperationID: "retained-pin", SelectedGenerationID: manifest.GenerationID,
			AcceptedGenerationID: manifest.GenerationID, PayloadChecksum: manifest.Payload.Checksum,
			RequestedAt: manifest.GeneratedAt, AcceptedAt: manifest.GeneratedAt}}
	data, err := encodeFleetRecoveryWithPin(t.Context(), layers, &pin)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data = data
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(data)
	snapshot.Head = snapshot.Publication.nextHead()
	if err := ValidateFleetRecovery(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFleetReplay(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	record, err := readFleetRecovery(t.Context(), snapshot.Publication.Recovery.Data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(record.Pin, &pin) {
		t.Fatal("validation changed the retained pin receipt")
	}
	if err := ValidateFleetReplay(t.Context(), snapshot, WithSource(newStubSource("foreign-authority"))); err == nil {
		t.Fatal("accepted a pin under another source authority")
	}
	pin.Receipt.AcceptedGenerationID = "different"
	data, err = encodeFleetRecoveryWithPin(t.Context(), layers, &pin)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data = data
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(data)
	snapshot.Head = snapshot.Publication.nextHead()
	if ValidateFleetRecovery(t.Context(), snapshot) == nil {
		t.Fatal("accepted a pin for a different generation")
	}
}
