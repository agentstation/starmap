package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

func recoveryDiscoveryFixture(t *testing.T) (*stateStore, State) {
	t.Helper()
	store, err := newStateStore(t.Context(), Config{StateDirectory: t.TempDir(), Repository: testRepository, Channel: "catalog"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	digest := strings.Repeat("a", 64)
	tag, err := artifact.ReleaseTag(digest)
	if err != nil {
		t.Fatal(err)
	}
	state := State{Repository: testRepository, Channel: "catalog", Sequence: 42, ChannelETag: `W/"retained"`, ChannelChecksum: digest, UpdatedAt: now,
		Verified: ReleaseRef{Tag: tag, GenerationID: "retained-generation", CatalogDigest: "sha256:" + digest, VerifiedAt: now}}
	if err := store.saveSnapshot(t.Context(), state, nil); err != nil {
		t.Fatal(err)
	}
	return store, state
}

func TestDiscoveryInspectionPreservesReplayFloorAndLegacyChecksum(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		store, state := recoveryDiscoveryFixture(t)
		if legacy {
			state.ChannelChecksum = ""
			body, err := encodeState(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.path, body, 0600); err != nil {
				t.Fatal(err)
			}
		}
		before, err := os.ReadFile(store.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := InspectStateDirectory(t.Context(), filepath.Dir(store.path)); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(store.path)
		if err != nil || string(before) != string(after) {
			t.Fatal("inspection changed the replay floor")
		}
		restored, err := store.load()
		if err != nil || restored.Sequence != 42 || restored.Verified != state.Verified {
			t.Fatalf("retained state changed: %v", err)
		}
	}
}

func TestDiscoveryInspectionRefusesInvalidRecords(t *testing.T) {
	for _, mode := range []string{"schema", "zero-sequence", "digest", "tag", "time", "identity", "name", "duplicate", "unknown", "oversize", "missing", "canceled", "nil"} {
		t.Run(mode, func(t *testing.T) {
			store, state := recoveryDiscoveryFixture(t)
			selected := filepath.Dir(store.path)
			ctx := t.Context()
			switch mode {
			case "schema":
				state.SchemaVersion = 99
			case "zero-sequence":
				state.Sequence = 0
			case "digest":
				state.Verified.CatalogDigest = "invalid"
			case "tag":
				state.Verified.Tag = "untrusted-tag"
			case "time":
				state.UpdatedAt = state.Verified.VerifiedAt.Add(-time.Second)
			case "identity":
				state.Channel = " catalog "
			case "missing":
				selected = filepath.Join(t.TempDir(), "absent")
			case "nil":
				ctx = nil
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if mode != "schema" {
				state.SchemaVersion = StateSchemaVersion
			}
			raw, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			if mode == "duplicate" {
				raw = append([]byte("{\"sequence\": 1,"), raw[1:]...)
			}
			if mode == "unknown" {
				raw = append([]byte("{\"unknown\":true,"), raw[1:]...)
			}
			if mode == "oversize" {
				raw = []byte(strings.Repeat(" ", maxStateBytes+1))
			}
			if err := os.WriteFile(store.path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "name" {
				digest := sha256.Sum256([]byte("another repository"))
				if err := os.Rename(store.path, filepath.Join(selected, hex.EncodeToString(digest[:])+".json")); err != nil {
					t.Fatal(err)
				}
			}
			if err := InspectStateDirectory(ctx, selected); err == nil {
				t.Fatal("invalid discovery state accepted")
			}
		})
	}
}

func TestUnsupportedDiscoverySchemaDoesNotResetReplayFloor(t *testing.T) {
	store, state := recoveryDiscoveryFixture(t)
	state.SchemaVersion = StateSchemaVersion + 1
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.load(); err == nil {
		t.Fatal("unsupported schema silently reset the replay floor")
	}
	after, err := os.ReadFile(store.path)
	if err != nil || string(after) != string(raw) {
		t.Fatal("schema refusal changed the retained record")
	}
}
