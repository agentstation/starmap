package acquisition_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/acquisition"
)

func TestPublicCredentialPolicyInspectionPreservesRetainedFamily(t *testing.T) {
	for _, product := range []acquisition.CredentialProduct{acquisition.CredentialProductStarmap, acquisition.CredentialProductStarport} {
		t.Run(string(product), func(t *testing.T) {
			state := acquisition.CredentialPolicyState{Directory: filepath.Join(t.TempDir(), "policy"), Product: "host", DeploymentID: "deployment", InstanceID: "replica", LegacyInstallation: true}
			_, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{Product: product, State: &state, Lookup: func(string) (string, bool) {
				t.Fatal("inspection read ambient credentials")
				return "", false
			}})
			if err != nil {
				t.Fatal(err)
			}
			state.LegacyInstallation = false
			if err := acquisition.InspectCredentialPolicyState(t.Context(), product, state); err != nil {
				t.Fatal(err)
			}
			other := acquisition.CredentialProductStarmap
			if product == other {
				other = acquisition.CredentialProductStarport
			}
			if err := acquisition.InspectCredentialPolicyState(t.Context(), other, state); err == nil {
				t.Fatal("wrong policy family accepted")
			}
			state.InstanceID = "another"
			if err := acquisition.InspectCredentialPolicyState(t.Context(), product, state); err == nil {
				t.Fatal("wrong owner accepted")
			}
		})
	}
}

func TestPublicCredentialPolicyInspectionDoesNotCreateState(t *testing.T) {
	state := acquisition.CredentialPolicyState{Directory: filepath.Join(t.TempDir(), "missing"), Product: "starport", DeploymentID: "deployment", InstanceID: "replica"}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, selected := range []context.Context{nil, ctx, t.Context()} {
		if err := acquisition.InspectCredentialPolicyState(selected, acquisition.CredentialProductStarport, state); err == nil {
			t.Fatal("absent policy accepted")
		}
	}
	if err := acquisition.InspectCredentialPolicyState(t.Context(), "unknown", state); err == nil {
		t.Fatal("unknown product accepted")
	}
	if _, err := os.Stat(state.Directory); !os.IsNotExist(err) {
		t.Fatalf("inspection created state: %v", err)
	}
}
