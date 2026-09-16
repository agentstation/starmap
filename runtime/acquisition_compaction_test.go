package runtime

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcquisitionCompactionRejectsInvalidInput(t *testing.T) {
	observation := manualProviderObservation(t, 200, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC))
	for _, name := range []string{"nil-context", "canceled", "duplicate", "changed-evidence", "history-limit"} {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			inputs := []sources.Observation{observation}
			switch name {
			case "nil-context":
				ctx = nil
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "duplicate":
				inputs = append(inputs, observation)
			case "changed-evidence":
				inputs[0].ObservedAt = inputs[0].ObservedAt.Add(time.Second)
			case "history-limit":
				inputs = make([]sources.Observation, maxManualHistoryBatches+1)
			}
			selected, err := compactAcquisitionHistory(ctx, inputs)
			if err == nil || selected != nil {
				t.Fatal("invalid input produced a retained history")
			}
			if name == "canceled" && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost its identity: %v", err)
			}
		})
	}
}

func TestAcquisitionCompactionReturnsOwnedOriginalEvidence(t *testing.T) {
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	binding := scopedProviderLayer(t, "account", "1", at).Receipt.ProviderBinding
	var inputs []sources.Observation
	for index := range 5 {
		inputs = append(inputs, providerResetObservation(t, sources.ProvidersID, manualProviderObservation(t, 200, at).Catalog, at.Add(time.Duration(index)*time.Hour), binding))
	}
	selected, err := compactAcquisitionHistory(t.Context(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].ID != inputs[0].ID || selected[1].ID != inputs[4].ID {
		t.Fatal("stable inventories lost their first or latest original evidence")
	}
	for _, observation := range selected {
		if err := observation.Validate(); err != nil {
			t.Fatal("compaction changed original evidence", err)
		}
	}
	selected[0].ProviderBinding.AccountID = "caller-change"
	selected[0].ID = "caller-change"
	if inputs[0].ProviderBinding.AccountID != binding.AccountID || inputs[0].ID == "caller-change" {
		t.Fatal("caller changed accepted input")
	}
}
