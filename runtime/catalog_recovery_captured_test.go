package runtime

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type capturedRecoveryFixture struct {
	name       string
	descriptor []byte
	baseline   catalogRecoveryBaseline
	generation catalogs.Generation
	original   CatalogRecovery
	record     localCatalogRecovery
}

func capturedRecoveryBytes(t *testing.T, record localCatalogRecovery) (string, []byte) {
	t.Helper()
	encoded, err := json.Marshal(record, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
	if err != nil {
		t.Fatal(err)
	}
	data, err := compressFleetRecovery(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return record.ManifestChecksum + "-" + fleetRecoveryChecksum(data) + ".json.gz", data
}

func newCapturedRecoveryFixture(t *testing.T) capturedRecoveryFixture {
	t.Helper()
	request, _, _ := materializationFixture(t)
	original := request.Inputs[0]
	inputs, err := readFleetRecovery(t.Context(), original.Recovery.Inputs.Data)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := encodeCatalogRecoveryBaseline(inputs.Baseline)
	if err != nil {
		t.Fatal(err)
	}
	inputs.Baseline = catalogs.Generation{}
	record := localCatalogRecovery{Version: localCatalogRecoveryVersion, ManifestChecksum: original.Recovery.ManifestChecksum, GenerationID: original.Generation.Manifest.GenerationID, PayloadChecksum: original.Generation.Manifest.Payload.Checksum, BaselineChecksum: baseline.manifestChecksum, Inputs: inputs}
	name, descriptor := capturedRecoveryBytes(t, record)
	return capturedRecoveryFixture{name, descriptor, baseline, original.Generation, original.Recovery, record}
}

func (f capturedRecoveryFixture) readBaseline(ctx context.Context, name string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if name != f.baseline.manifestChecksum+".json.gz" || limit != MaxFleetRecoveryBytes {
		return nil, errors.New("wrong original baseline request")
	}
	return f.baseline.data, nil
}

func TestCapturedCatalogRecoveryPreservesOriginalInputs(t *testing.T) {
	f := newCapturedRecoveryFixture(t)
	calls := 0
	checked, err := InspectCapturedCatalogRecovery(t.Context(), f.name, f.descriptor, func(ctx context.Context, name string, limit int64) ([]byte, error) {
		calls++
		return f.readBaseline(ctx, name, limit)
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("baseline reads: %d", calls)
	}
	want := CapturedCatalogBinding{f.original.ManifestChecksum, f.generation.Manifest.GenerationID, f.generation.Manifest.Payload.Checksum}
	if checked.Binding() != want {
		t.Fatal("original generation binding changed")
	}
	recovery, err := checked.RecoveryFor(t.Context(), f.generation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovery, f.original) {
		t.Fatal("original semantic inputs changed")
	}
	// Caller bytes and returned bytes cannot change the checked private inputs.
	clear(f.descriptor)
	clear(f.baseline.data)
	clear(recovery.Inputs.Data)
	again, err := checked.RecoveryFor(t.Context(), f.generation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, f.original) {
		t.Fatal("caller changed retained inputs")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v %p", checked, checked, checked), f.original.Inputs.Checksum) {
		t.Fatal("diagnostics expose inputs")
	}
	changed := f.generation.Copy()
	changed.Manifest.GenerationID += "-other"
	if _, err := checked.RecoveryFor(t.Context(), changed); err == nil {
		t.Fatal("unrelated generation accepted")
	}
	var absent *CapturedCatalogRecovery
	if absent.Binding() != (CapturedCatalogBinding{}) {
		t.Fatal("nil binding")
	}
	if _, err := absent.RecoveryFor(t.Context(), f.generation); err == nil {
		t.Fatal("nil proof accepted")
	}
	if _, err := (&CapturedCatalogRecovery{}).RecoveryFor(t.Context(), f.generation); err == nil {
		t.Fatal("unchecked zero proof accepted")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := checked.RecoveryFor(canceled, f.generation); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := checked.RecoveryFor(nil, f.generation); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestCapturedCatalogRecoveryRejectsChangedArchiveInputs(t *testing.T) {
	f := newCapturedRecoveryFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(*localCatalogRecovery)
	}{
		{"schema", func(r *localCatalogRecovery) { r.Version++ }},
		{"manifest", func(r *localCatalogRecovery) { r.ManifestChecksum = "invalid" }},
		{"generation", func(r *localCatalogRecovery) { r.GenerationID = "" }},
		{"payload", func(r *localCatalogRecovery) { r.PayloadChecksum = "sha256:invalid" }},
		{"baseline identity", func(r *localCatalogRecovery) { r.BaselineChecksum = strings.Repeat("a", 64) }},
		{"inlined baseline", func(r *localCatalogRecovery) { r.Inputs.Baseline = f.generation }},
		{"input version", func(r *localCatalogRecovery) { r.Inputs.Version++ }},
		{"invalid provider", func(r *localCatalogRecovery) { r.Inputs.Providers = []ProviderLayer{{}} }},
		{"invalid source", func(r *localCatalogRecovery) { r.Inputs.Source = &sourceLayer{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := f.record
			tc.mutate(&record)
			name, descriptor := capturedRecoveryBytes(t, record)
			if _, err := InspectCapturedCatalogRecovery(t.Context(), name, descriptor, f.readBaseline); err == nil {
				t.Fatal("invalid original accepted")
			}
		})
	}
	for _, name := range []string{"../" + f.name, f.name + ".extra", strings.Repeat("b", 64) + f.name[64:]} {
		if _, err := InspectCapturedCatalogRecovery(t.Context(), name, f.descriptor, f.readBaseline); err == nil {
			t.Fatal("invalid filename accepted")
		}
	}
	for _, data := range [][]byte{nil, []byte("not gzip"), append(bytes.Clone(f.descriptor), 1)} {
		if _, err := InspectCapturedCatalogRecovery(t.Context(), f.name, data, f.readBaseline); err == nil {
			t.Fatal("invalid descriptor accepted")
		}
	}
	for _, data := range [][]byte{nil, []byte("not gzip"), append(bytes.Clone(f.baseline.data), 1)} {
		if _, err := InspectCapturedCatalogRecovery(t.Context(), f.name, f.descriptor, func(context.Context, string, int64) ([]byte, error) { return data, nil }); err == nil {
			t.Fatal("invalid baseline accepted")
		}
	}
	// A different valid baseline must also fail its original immutable identity.
	other := f.generation.Copy()
	other.Manifest.GenerationID += "-baseline"
	encoded, err := encodeCatalogRecoveryBaseline(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectCapturedCatalogRecovery(t.Context(), f.name, f.descriptor, func(context.Context, string, int64) ([]byte, error) { return encoded.data, nil }); err == nil {
		t.Fatal("replaced baseline accepted")
	}
	unknownEncoded, err := json.Marshal(f.record, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
	if err != nil {
		t.Fatal(err)
	}
	unknownEncoded = append(unknownEncoded[:len(unknownEncoded)-1], []byte(`,"unknown_member":true}`)...)
	compressed, err := compressFleetRecovery(unknownEncoded)
	if err != nil {
		t.Fatal(err)
	}
	name := f.record.ManifestChecksum + "-" + fleetRecoveryChecksum(compressed) + ".json.gz"
	if _, err := InspectCapturedCatalogRecovery(t.Context(), name, compressed, f.readBaseline); err == nil {
		t.Fatal("unknown descriptor member accepted")
	}
	sentinel := errors.New("archive read failed")
	if _, err := InspectCapturedCatalogRecovery(t.Context(), f.name, f.descriptor, func(context.Context, string, int64) ([]byte, error) { return nil, sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if _, err := InspectCapturedCatalogRecovery(t.Context(), f.name, f.descriptor, nil); err == nil {
		t.Fatal("missing reader accepted")
	}
}

func TestCapturedCatalogRecoveryChecksContextBeforeArchiveRead(t *testing.T) {
	f := newCapturedRecoveryFixture(t)
	calls := 0
	reader := func(context.Context, string, int64) ([]byte, error) { calls++; return f.baseline.data, nil }
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := InspectCapturedCatalogRecovery(canceled, f.name, f.descriptor, reader); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := InspectCapturedCatalogRecovery(nil, f.name, f.descriptor, reader); err == nil {
		t.Fatal("nil context accepted")
	}
	if calls != 0 {
		t.Fatal("invalid context reached archive reader")
	}
}
