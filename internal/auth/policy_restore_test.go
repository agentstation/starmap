package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestInspectFilePolicyStorePreservesAcceptedAndLegacyRecords(t *testing.T) {
	for _, initial := range []EnvironmentPolicy{EnvironmentPolicyLegacy, EnvironmentPolicyCurrent, EnvironmentPolicyStarportLegacy, EnvironmentPolicyStarportCurrent} {
		t.Run(string(initial), func(t *testing.T) {
			owner := PolicyOwner{Product: "host", Deployment: "deployment", Instance: "replica"}
			path := filepath.Join(t.TempDir(), "policy")
			store, err := OpenFilePolicyStore(t.Context(), path, owner, initial)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Accept(t.Context(), "openai"); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(path, policyRecordName))
			if err != nil {
				t.Fatal(err)
			}
			if err := InspectFilePolicyStore(t.Context(), path, owner, initial.current()); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(path, policyRecordName))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("inspection changed the default: %v", err)
			}
			for provider, want := range map[catalogs.ProviderID]EnvironmentPolicy{"openai": initial.current(), "anthropic": initial} {
				if got, err := store.Policy(t.Context(), provider); err != nil || got != want {
					t.Fatalf("%s policy = %s, %v", provider, got, err)
				}
			}
		})
	}
}

func TestInspectFilePolicyStoreRefusesInvalidRetainedState(t *testing.T) {
	for _, mode := range []string{"owner", "family", "missing-default", "default-provider", "provider-name", "provider-legacy", "unknown-field", "oversize", "extra-file", "extra-directory", "symlink", "pending-publication"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy")
			owner := PolicyOwner{Product: "starport", Deployment: "deployment", Instance: "replica"}
			store, err := OpenFilePolicyStore(t.Context(), path, owner, EnvironmentPolicyStarportLegacy)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Accept(t.Context(), "openai"); err != nil {
				t.Fatal(err)
			}
			current := EnvironmentPolicyStarportCurrent
			name := policyRecordName
			record := policyRecord{SchemaVersion: 1, Owner: owner, Policy: EnvironmentPolicyStarportLegacy}
			switch mode {
			case "owner":
				owner.Instance = "different"
			case "family":
				current = EnvironmentPolicyCurrent
			case "missing-default":
				err = os.Remove(filepath.Join(path, name))
			case "default-provider":
				record.Provider = "openai"
			case "provider-name":
				err = os.Rename(filepath.Join(path, providerPolicyName("openai")), filepath.Join(path, providerPolicyName("another")))
			case "provider-legacy":
				name, record.Provider = providerPolicyName("openai"), "openai"
			case "extra-file":
				err = os.WriteFile(filepath.Join(path, "unknown"), []byte("{}"), 0o600)
			case "extra-directory":
				err = os.Mkdir(filepath.Join(path, "unknown"), 0o700)
			case "symlink":
				err = os.Symlink(filepath.Join(path, name), filepath.Join(path, "alias"))
				if err != nil {
					t.Skipf("host refuses symlink fixture: %v", err)
				}
			case "pending-publication":
				_, err = privatefiles.NewDirectory(filepath.Join(path, privatefiles.PublicationDirectoryName))
				if err == nil {
					err = os.WriteFile(filepath.Join(path, privatefiles.PublicationDirectoryName, "pending.jsonl"), []byte("{}"), 0o600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "default-provider" || mode == "provider-legacy" || mode == "unknown-field" || mode == "oversize" {
				body, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "unknown-field" {
					body = append(body[:len(body)-1], []byte(",\"extra\":true}")...)
				}
				if mode == "oversize" {
					body = bytes.Repeat([]byte("x"), policyRecordLimit+1)
				}
				if err := os.WriteFile(filepath.Join(path, name), body, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := InspectFilePolicyStore(t.Context(), path, owner, current); err == nil {
				t.Fatal("invalid retained state accepted")
			}
		})
	}
}

func TestInspectFilePolicyStoreDoesNotInitialize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	owner := PolicyOwner{Product: "host", Deployment: "deployment", Instance: "replica"}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, selected := range []context.Context{nil, ctx, t.Context()} {
		if err := InspectFilePolicyStore(selected, path, owner, EnvironmentPolicyCurrent); err == nil {
			t.Fatal("missing state accepted")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("inspection created state: %v", err)
		}
	}
}
