package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
)

func TestConfigPathsInspectionReportsFilesWithoutInitialization(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	root := filepath.Join(home, "config")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "config.yaml")
	contents := []byte("catalog_source: embedded\n")
	if err := os.WriteFile(file, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	a := NewForCommand("test", "test", "test", "test")
	command := a.createRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"config", "paths", "--inspect", "--output", "json"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Inspection struct {
			Complete     bool                                     `json:"complete"`
			Observations []struct{ ID, Path, State, Kind string } `json:"observations"`
		} `json:"inspection"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range result.Inspection.Observations {
		if item.ID == "configuration" {
			found = item.Path == file && item.State == "present" && item.Kind == "file"
		}
	}
	if !result.Inspection.Complete || !found {
		t.Fatal("inspection omits the selected configuration file")
	}
	if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
		t.Fatal("inspection initialized catalog or credential state")
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 1 || entries[0].Name() != "config" {
		t.Fatal("inspection created product directories")
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(contents, after) {
		t.Fatal("inspection changed configuration bytes")
	}
}

func TestFileInspectionReportsActiveOwnerAndLiteralWorkspace(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	workspace := filepath.Join(home, "work[1]")
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
		catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false",
	}, CatalogPath: workspace}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.closeRuntime() })
	stage := filepath.Join(home, ".work[1].candidate-test")
	if err := os.Mkdir(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(stage, "payload")
	if err := os.WriteFile(payload, []byte("retained staging bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := a.InspectFiles(t.Context(), 10000)
	if err != nil {
		t.Fatal(err)
	}
	wantBinding := "matches"
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		wantBinding = "unverified"
	}
	ownerFound, stageFound := false, false
	for _, item := range report.Inspection.Observations {
		if item.ID == "runtime-owner" {
			ownerFound = item.OwnerBinding == wantBinding && item.State == "present"
		}
		if item.Path == payload {
			stageFound = item.State == "present"
		}
	}
	if !report.Inspection.Complete || !ownerFound || !stageFound {
		t.Fatalf("complete=%t, owner=%t, literal workspace stage=%t", report.Inspection.Complete, ownerFound, stageFound)
	}
	after, err := os.ReadFile(payload)
	if err != nil || string(after) != "retained staging bytes" {
		t.Fatal("inspection changed staging bytes")
	}
}
