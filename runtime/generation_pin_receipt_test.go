package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestGenerationPinRecordsOneAcceptanceAcrossRestart(t *testing.T) {
	store := storage.NewMemory()
	selected := aliasGeneration(t, "receipt-selected")
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	newer := aliasGeneration(t, "receipt-newer")
	if err := store.Commit(t.Context(), newer, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	directory := privateRuntimeDirectory(t)
	options := []Option{WithCatalogSource("embedded"), WithStateDirectory(directory), WithGenerationPin(selected.Manifest.GenerationID),
		WithClientOptions(starmap.WithCatalogStore(store))}
	var prior []byte
	for range 2 {
		r := openTestRuntime(t, options...)
		raw, err := os.ReadFile(filepath.Join(directory, "catalog-runtime", "generation-pin.json"))
		if err != nil {
			t.Fatal(err)
		}
		var record struct {
			Phase   string `json:"phase"`
			Receipt struct {
				OperationID          string `json:"operation_id"`
				SelectedGenerationID string `json:"selected_generation_id"`
				AcceptedGenerationID string `json:"accepted_generation_id"`
				PreviousGenerationID string `json:"previous_generation_id"`
			} `json:"receipt"`
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if record.Phase != "accepted" || record.Receipt.OperationID == "" || record.Receipt.SelectedGenerationID != selected.Manifest.GenerationID ||
			record.Receipt.AcceptedGenerationID != r.State().GenerationID || record.Receipt.PreviousGenerationID != newer.Manifest.GenerationID {
			t.Fatal("pin acceptance does not identify the selected artifact and predecessor")
		}
		if prior != nil && !bytes.Equal(raw, prior) {
			t.Fatal("restart replaced the acceptance event")
		}
		prior = raw
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOriginGenerationPinPublishesPriorPayloadAtNewRevision(t *testing.T) {
	store := storage.NewMemory()
	source := newStubSource("origin-pin")
	first := aliasGeneration(t, "origin-input-first")
	source.replies = []SourceRead{aliasRead(first)}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"),
		WithSource(source), WithSourceRefreshMode("manual"), WithAuthorityOrigin(store, originTestConfig())}
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	selected, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	next := first.Copy()
	next.Manifest.GenerationID = "origin-input-second"
	next.Payload = bytes.ReplaceAll(next.Payload, []byte(`"Current"`), []byte(`"Changed"`))
	next.Manifest.Payload = catalogs.DescribeCatalogPayload(next.Payload)
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	source.replies = []SourceRead{aliasRead(next)}
	source.mu.Unlock()
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	previous := r.State()
	if previous.PayloadChecksum == selected.Manifest.Payload.Checksum {
		t.Fatal("fixture did not change the catalog")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	options = append(options, WithGenerationPin(selected.Manifest.GenerationID))
	var activated string
	for attempt := range 3 {
		selectedOptions := options
		if attempt == 2 {
			selectedOptions = append(slices.Clone(options), WithStateDirectory(privateRuntimeDirectory(t)))
		}
		pinned := openTestRuntime(t, selectedOptions...)
		state := pinned.State()
		if state.GenerationID == selected.Manifest.GenerationID || state.AuthorityHead.Sequence != previous.AuthorityHead.Sequence+1 ||
			state.PayloadChecksum != selected.Manifest.Payload.Checksum {
			t.Fatal("origin rollback did not bind the prior payload to a new authority revision")
		}
		if activated != "" && state.GenerationID != activated {
			t.Fatal("restart issued another rollback revision")
		}
		activated = state.GenerationID
		permission, err := pinned.ReadPermission(t.Context())
		if err != nil || permission.Head != state.AuthorityHead {
			t.Fatalf("issuer differs from the rollback: %v", err)
		}
		if err := pinned.Close(); err != nil {
			t.Fatal(err)
		}
	}
	unpinned := openTestRuntime(t, append(options, WithGenerationPin(""))...)
	if unpinned.State().PayloadChecksum != previous.PayloadChecksum || unpinned.State().AuthorityHead.Sequence != previous.AuthorityHead.Sequence+2 {
		t.Fatal("origin unpin did not restore retained inputs at the next revision")
	}
}

type pinCommitFaultStore struct {
	*storage.Memory
	reject      atomic.Bool
	lost        atomic.Bool
	afterCommit func() error
}

func (s *pinCommitFaultStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.reject.Swap(false) {
		return fs.ErrPermission
	}
	if err := s.Memory.Commit(ctx, generation, expected); err != nil {
		return err
	}
	if s.lost.Swap(false) {
		return fs.ErrPermission
	}
	if s.afterCommit != nil {
		return s.afterCommit()
	}
	return nil
}

func TestGenerationPinRecoversItsRecordedPublication(t *testing.T) {
	for _, origin := range []bool{false, true} {
		name := "ordinary"
		if origin {
			name = "origin"
		}
		t.Run(name, func(t *testing.T) {
			for _, fault := range []string{"before-commit", "lost-reply"} {
				t.Run(fault, func(t *testing.T) {
					store := &pinCommitFaultStore{Memory: storage.NewMemory()}
					selected, newer := aliasGeneration(t, "pin-input-old"), aliasGeneration(t, "pin-input-new")
					if origin {
						var err error
						selected, err = permission.PrepareGeneration(selected, permission.GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1})
						if err != nil {
							t.Fatal(err)
						}
						newer, err = permission.PrepareGeneration(newer, permission.GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 2})
						if err != nil {
							t.Fatal(err)
						}
					}
					if err := store.Commit(t.Context(), selected, ""); err != nil {
						t.Fatal(err)
					}
					if err := store.Commit(t.Context(), newer, selected.Manifest.GenerationID); err != nil {
						t.Fatal(err)
					}
					directory := privateRuntimeDirectory(t)
					options := []Option{WithCatalogSource("embedded"), WithStateDirectory(directory), WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithGenerationPin(selected.Manifest.GenerationID)}
					if origin {
						options = append(options, WithAuthorityOrigin(store, originTestConfig()))
					} else {
						options = append(options, WithClientOptions(starmap.WithCatalogStore(store)))
					}
					store.reject.Store(fault == "before-commit")
					store.lost.Store(fault == "lost-reply")
					if unexpected, err := Open(t.Context(), options...); err == nil {
						_ = unexpected.Close()
						t.Fatal("publication fault reached readiness")
					}
					raw, err := os.ReadFile(filepath.Join(directory, "catalog-runtime", "generation-pin.json"))
					if err != nil {
						t.Fatal(err)
					}
					var pending generationPinRecord
					if err := json.Unmarshal(raw, &pending); err != nil {
						t.Fatal(err)
					}
					if pending.Phase != pinPrepared || pending.Receipt.PreviousGenerationID != newer.Manifest.GenerationID {
						t.Fatal("publication fault lost its original predecessor")
					}
					current, err := store.Current(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					expected := newer.Manifest.GenerationID
					if fault == "lost-reply" {
						expected = pending.Receipt.AcceptedGenerationID
					}
					if current.Manifest.GenerationID != expected {
						t.Fatal("fault did not isolate the selected publication boundary")
					}
					recovered := openTestRuntime(t, options...)
					receipt, durable := recovered.PinAcceptance()
					if !durable || receipt.OperationID != pending.Receipt.OperationID || receipt.AcceptedGenerationID != pending.Receipt.AcceptedGenerationID || recovered.State().GenerationID != receipt.AcceptedGenerationID {
						t.Fatal("recovery replaced the operation or generation identity")
					}
					if origin && recovered.State().AuthorityHead.Sequence != 3 {
						t.Fatal("recovery issued an extra authority sequence")
					}
				})
			}
		})
	}
}

func TestGenerationPinChecksAuthorityBindingAndRecordSchema(t *testing.T) {
	store := storage.NewMemory()
	generation := aliasGeneration(t, "bound-pin")
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	directory := privateRuntimeDirectory(t)
	source := newStubSource("accepted-source")
	options := []Option{WithCatalogSource("embedded"), WithSource(source), WithStateDirectory(directory),
		WithGenerationPin(generation.Manifest.GenerationID), WithClientOptions(starmap.WithCatalogStore(store))}
	original := openTestRuntime(t, options...)
	first, durable := original.PinAcceptance()
	if !durable || first.OperationID == "" {
		t.Fatal("initial pin has no durable acceptance")
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	replacement := newStubSource("different-source")
	if unexpected, err := Open(t.Context(), append(slices.Clone(options), WithSource(replacement))...); err == nil {
		_ = unexpected.Close()
		t.Fatal("changed source retained an incompatible pin")
	}
	unchanged := openTestRuntime(t, append(slices.Clone(options), WithSourceRepository("unused/repository"))...)
	retained, _ := unchanged.PinAcceptance()
	if retained.OperationID != first.OperationID {
		t.Fatal("an unused source setting replaced acceptance")
	}
	if err := unchanged.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "catalog-runtime", generationPinRecordFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{append(bytes.Clone(raw), []byte(` {}`)...), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":2`), 1), bytes.Replace(raw, []byte(`"phase":"accepted"`), []byte(`"phase":"unknown"`), 1)} {
		if bytes.Equal(raw, malformed) {
			t.Fatal("fixture did not alter its acceptance record")
		}
		if err := os.WriteFile(path, malformed, privatefiles.FileMode); err != nil {
			t.Fatal(err)
		}
		if unexpected, err := Open(t.Context(), options...); err == nil {
			_ = unexpected.Close()
			t.Fatal("malformed acceptance reached readiness")
		}
	}
	if err := os.WriteFile(path, raw, privatefiles.FileMode); err != nil {
		t.Fatal(err)
	}
	released := openTestRuntime(t, append(slices.Clone(options), WithGenerationPin(""))...)
	if receipt, _ := released.PinAcceptance(); receipt.OperationID != "" {
		t.Fatal("unpin reports an active acceptance")
	}
	if err := released.Close(); err != nil {
		t.Fatal(err)
	}
	repinned := openTestRuntime(t, options...)
	next, _ := repinned.PinAcceptance()
	if next.OperationID == first.OperationID {
		t.Fatal("a new pin reused the released operation")
	}
}

func TestGenerationPinRecoversFailedAcceptanceWrite(t *testing.T) {
	store := &pinCommitFaultStore{Memory: storage.NewMemory()}
	selected, previous := aliasGeneration(t, "write-selected"), aliasGeneration(t, "write-previous")
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), previous, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	directory := privateRuntimeDirectory(t)
	path := filepath.Join(directory, "catalog-runtime", generationPinRecordFile)
	backup := path + ".test-prepared"
	store.afterCommit = func() error {
		if err := os.Rename(path, backup); err != nil {
			return err
		}
		return os.Mkdir(path, privatefiles.DirectoryMode)
	}
	options := []Option{WithCatalogSource("embedded"), WithStateDirectory(directory), WithGenerationPin(selected.Manifest.GenerationID), WithClientOptions(starmap.WithCatalogStore(store))}
	if unexpected, err := Open(t.Context(), options...); err == nil {
		_ = unexpected.Close()
		t.Fatal("failed acceptance write reached readiness")
	}
	current, err := store.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != selected.Manifest.GenerationID {
		t.Fatal("acceptance write failure reversed catalog publication")
	}
	raw, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	var prepared generationPinRecord
	if err := json.Unmarshal(raw, &prepared); err != nil {
		t.Fatal(err)
	}
	if prepared.Phase != pinPrepared {
		t.Fatal("fixture did not preserve the prepared event")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, path); err != nil {
		t.Fatal(err)
	}
	store.afterCommit = nil
	recovered := openTestRuntime(t, options...)
	receipt, durable := recovered.PinAcceptance()
	if !durable || receipt.OperationID != prepared.Receipt.OperationID || recovered.State().GenerationID != selected.Manifest.GenerationID {
		t.Fatal("recovery replaced an accepted operation")
	}
}
