package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestFileManifestReportsAccessAndRetentionPolicies(t *testing.T) {
	clearCatalogEnvironment(t)
	home := t.TempDir()
	t.Setenv("STARMAP_HOME", home)
	a := NewForCommand("test", "test", "test", "test")
	report, err := a.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Files []struct {
			ID     string
			Policy struct {
				Selectors                                 []string
				Applicability, Access, Retention, Removal string
			}
		}
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, entry := range decoded.Files {
		p := entry.Policy
		if len(p.Selectors) == 0 || p.Applicability == "" || p.Access == "" || p.Retention == "" || p.Removal == "" {
			t.Errorf("file role %s lacks a complete policy: %+v", entry.ID, p)
		}
		if (entry.ID == "configuration" || entry.ID == "catalog-store") && p.Access != "owner-only" {
			t.Errorf("%s must require private access", entry.ID)
		}
		if entry.ID == "baseline" && p.Access != "public-read" {
			t.Error("static embedded baseline must permit explicit shared read access")
		}
		if entry.ID == "workspace-backup" && p.Retention != "recovery" {
			t.Error("workspace backup must remain recovery state")
		}
	}
	if len(decoded.Files) == 0 {
		t.Fatal("file policy report is empty")
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("policy report created files")
	}
	if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
		t.Fatal("policy report initialized runtime state")
	}
	if _, err := os.Stat(filepath.Join(home, "config")); !os.IsNotExist(err) {
		t.Fatal("policy report created a configuration root")
	}
}

func TestConfigPathsReportsPoliciesAcrossFormats(t *testing.T) {
	for _, kind := range []string{"json", "yaml", "wide"} {
		t.Run(kind, func(t *testing.T) {
			clearCatalogEnvironment(t)
			home := t.TempDir()
			t.Setenv("STARMAP_HOME", home)
			a := NewForCommand("test", "test", "test", "test")
			command := a.createRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{"config", "paths", "--output", kind})
			if err := command.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			text := strings.ToLower(output.String())
			if !strings.Contains(text, "owner-only") || !strings.Contains(text, "recovery") {
				t.Fatalf("%s omits access or retention policy", kind)
			}
			if kind != "wide" && (!strings.Contains(text, "selectors") || !strings.Contains(text, "removal")) {
				t.Fatalf("%s omits policy detail", kind)
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatal("policy command created files")
			}
		})
	}
}

func TestFilePoliciesCoverExternalInputsAndRejectUnknownRoles(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	a := NewForCommand("test", "test", "test", "test")
	first, err := a.FileManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range first.External {
		p := entry.Policy
		if len(p.Selectors) == 0 || p.Applicability == "" || p.Access == "" || p.Retention == "" || p.Removal == "" {
			t.Errorf("external role %s lacks a complete policy", entry.ID)
		}
	}
	first.Files[0].Policy.Selectors[0] = "caller edit"
	second, err := a.FileManifest()
	if err != nil || second.Files[0].Policy.Selectors[0] == "caller edit" {
		t.Fatal("policy report exposes shared selector storage", err)
	}
	for _, report := range []productpaths.FileManifest{
		{Files: []productpaths.FileEntry{{ID: "unknown"}}},
		{External: []productpaths.ExternalFiles{{ID: "unknown"}}},
	} {
		if err := applyFilePolicies(&report); err == nil {
			t.Fatal("unknown file role received an implicit policy")
		}
	}
}

func TestMigrationFilePolicyNamesExistingFlags(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	a := NewForCommand("test", "test", "test", "test")
	command, _, err := a.createRootCommand().Find([]string{"migrate", "runtime", "prepare"})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := externalFilePolicy("runtime-migration")
	if err != nil {
		t.Fatal(err)
	}
	for _, selector := range policy.Selectors {
		_, name, found := strings.Cut(selector, " --")
		if !found || command.Flags().Lookup(name) == nil {
			t.Errorf("migration policy names no selectable flag: %s", selector)
		}
	}
}
