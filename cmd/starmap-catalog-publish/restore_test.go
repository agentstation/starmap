package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
)

func TestPublicationCommandRestoresExactInputsWithoutAcquisition(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"one","object":"model"}]}`))
	}))
	defer server.Close()
	t.Setenv("PUBLICATION_FIXTURE_KEY", "fixture")
	root := t.TempDir()
	profile, state, checksum := commandInputs(t, root, server.URL)
	prepared := commandResult(t, []string{"-profile", profile, "-state", state, "-state-checksum", checksum,
		"-publisher-id", "command-publisher", "-run-id", "restore-one", "-output-dir", filepath.Join(root, "prepared")})
	if requests.Load() != 1 {
		t.Fatal("fixture did not acquire the original source")
	}
	t.Setenv("PUBLICATION_FIXTURE_KEY", "")
	args := []string{"-restore-receipt", prepared.ReceiptPath, "-restore-receipt-checksum", prepared.ReceiptChecksum,
		"-state", prepared.StatePath, "-state-checksum", prepared.StateChecksum,
		"-publisher-id", "command-publisher", "-run-id", "restore-one", "-output-dir", filepath.Join(root, "restored")}
	restored := commandResult(t, args)
	if restored.Status != "restored" || !restored.ReusedArtifact || restored.ArchiveChecksum != prepared.ArchiveChecksum || requests.Load() != 1 {
		t.Fatal("restore changed the artifact or started acquisition")
	}
	if mustRead(t, restored.StatePath) != mustRead(t, prepared.StatePath) || mustRead(t, restored.ReceiptPath) != mustRead(t, prepared.ReceiptPath) {
		t.Fatal("restore changed original publication bytes")
	}
	if commandResult(t, args) != restored || requests.Load() != 1 {
		t.Fatal("repeated restore changed its output or started acquisition")
	}
	for _, name := range []string{"receipt-digest", "checkpoint-digest", "run-id", "publisher-id", "profile", "baseline", "canceled"} {
		t.Run(name, func(t *testing.T) {
			invalidArgs := slices.Clone(args)
			destination := filepath.Join(root, name)
			invalidArgs[len(invalidArgs)-1] = destination
			ctx := t.Context()
			switch name {
			case "receipt-digest":
				invalidArgs[3] = checksum
			case "checkpoint-digest":
				invalidArgs[7] = prepared.ReceiptChecksum
			case "run-id":
				invalidArgs[11] = "another-run"
			case "publisher-id":
				invalidArgs[9] = "another-publisher"
			case "profile":
				invalidArgs = append(invalidArgs, "-profile", profile)
			case "baseline":
				invalidArgs = append(invalidArgs, "-baseline-embedded")
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			var output bytes.Buffer
			if err := run(ctx, invalidArgs, &output); err == nil || output.Len() != 0 {
				t.Fatal("invalid restore reported success")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("invalid restore staged output")
			}
			if requests.Load() != 1 {
				t.Fatal("invalid restore started acquisition")
			}
		})
	}
}
