package config

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/runtime"
)

func TestRetentionSettingsReachRuntime(t *testing.T) {
	settings, err := Parse(map[string]string{RetentionEnabled: "false", RetentionInterval: "2h", RetentionMaxGenerations: "7",
		RetentionMaxBytes: "123456", RetentionScanEntries: "99", RetentionInputMaxBytes: "789012"})
	if err != nil {
		t.Fatal(err)
	}
	options := append([]runtime.Option{runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false)}, settings.Options()...)
	r, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got := r.RetentionSnapshot()
	if got.Enabled || got.Interval != 2*time.Hour || got.MaxGenerations != 7 || got.MaxBytes != 123456 || got.ScanEntries != 99 || got.InputMaxBytes != 789012 {
		t.Fatalf("canonical settings did not reach runtime: %+v", got)
	}
	if !got.AttemptedAt.IsZero() {
		t.Fatal("disabled scheduling started maintenance")
	}
}

func TestRetentionSettingsRejectUnboundedValues(t *testing.T) {
	for name, values := range map[string][]string{
		RetentionEnabled: {"", "maybe"}, RetentionInterval: {"", "0", "-1h"},
		RetentionMaxGenerations: {"0", "-1"}, RetentionMaxBytes: {"0", "-1", "9223372036854775808"},
		RetentionScanEntries: {"0", "100001"}, RetentionInputMaxBytes: {"0", "-1"},
	} {
		for _, value := range values {
			if _, err := Parse(map[string]string{name: value}); err == nil {
				t.Errorf("accepted %s=%q", name, value)
			}
		}
	}
}
