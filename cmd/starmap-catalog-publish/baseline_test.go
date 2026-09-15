package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPublicationCommandSelectsCompiledBaselineAndBindsRetry(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	root := t.TempDir()
	profile, state, checksum := commandInputs(t, root, server.URL)
	policy := mustRead(t, profile)
	if !strings.Contains(policy, "enabled: true") {
		t.Fatal("fixture did not declare its source selection")
	}
	if err := os.WriteFile(profile, []byte(strings.Replace(policy, "enabled: true", "enabled: false", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-baseline-embedded", "-profile", profile, "-state", state, "-state-checksum", checksum,
		"-publisher-id", "command-publisher", "-run-id", "compiled-baseline", "-output-dir", filepath.Join(root, "output")}
	first := commandResult(t, args)
	compiled, err := bootstrap.Generation()
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		Baseline catalogs.Generation `json:"baseline"`
	}
	if err := json.Unmarshal([]byte(mustRead(t, first.StatePath)), &checkpoint); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(checkpoint.Baseline.Payload, compiled.Payload) || checkpoint.Baseline.Manifest.GenerationID != compiled.Manifest.GenerationID {
		t.Fatal("resume retained the prior baseline instead of the verified compiled input")
	}
	second := commandResult(t, args)
	if first != second || requests.Load() != 0 {
		t.Fatal("retry changed prepared output or acquired a disabled provider")
	}
	var output bytes.Buffer
	if err := run(t.Context(), args[1:], &output); err == nil || output.Len() != 0 {
		t.Fatal("same run accepted a different baseline policy")
	}
}
