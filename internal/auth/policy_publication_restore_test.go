package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func policyRestoreInputs(bodies map[string][]byte) (map[string]privatefiles.RetainedFile, privatefiles.RetainedRecordReader) {
	files := make(map[string]privatefiles.RetainedFile, len(bodies))
	for name, body := range bodies {
		sum := sha256.Sum256(body)
		files[name] = privatefiles.RetainedFile{Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}
	}
	return files, func(_ context.Context, name string, _ int64) ([]byte, error) {
		body, ok := bodies[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return body, nil
	}
}

func TestPolicyPublicationRestoreKeepsCompleteLegacyStagesInactive(t *testing.T) {
	owner := PolicyOwner{Product: "starport", Deployment: "deployment", Instance: "instance"}
	for _, provider := range []string{"", "openai"} {
		t.Run(provider, func(t *testing.T) {
			record := policyRecord{SchemaVersion: 1, Owner: owner, Policy: EnvironmentPolicyStarportCurrent, Provider: catalogs.ProviderID(provider)}
			body, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			name := policyRecordPrefix + strings.Repeat("A", 26)
			bodies := map[string][]byte{name: body}
			files, read := policyRestoreInputs(bodies)
			inactive, err := InspectPolicyPublications(t.Context(), owner, EnvironmentPolicyStarportCurrent, files, read)
			if err != nil || !reflect.DeepEqual(inactive, []string{name}) {
				t.Fatalf("inactive selection: %v %v", inactive, err)
			}
			if string(bodies[name]) != string(body) {
				t.Fatal("changed staging bytes")
			}
		})
	}
}

func TestPolicyPublicationRestoreRejectsUnownedLegacyStages(t *testing.T) {
	owner := PolicyOwner{Product: "starport", Deployment: "deployment", Instance: "instance"}
	for _, mode := range []string{"owner", "family", "partial", "name", "changed-bytes", "missing-reader", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			record := policyRecord{SchemaVersion: 1, Owner: owner, Policy: EnvironmentPolicyStarportCurrent}
			if mode == "owner" {
				record.Owner.Instance = "another"
			}
			if mode == "family" {
				record.Policy = EnvironmentPolicyCurrent
			}
			body, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "partial" {
				body = body[:len(body)-1]
			}
			name := policyRecordPrefix + strings.Repeat("A", 26)
			if mode == "name" {
				name = policyRecordPrefix + "operator-note"
			}
			bodies := map[string][]byte{name: body}
			files, read := policyRestoreInputs(bodies)
			if mode == "changed-bytes" {
				bodies[name] = append(body, ' ')
			}
			if mode == "missing-reader" {
				read = nil
			}
			ctx := t.Context()
			if mode == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := InspectPolicyPublications(ctx, owner, EnvironmentPolicyStarportCurrent, files, read); err == nil {
				t.Fatal("accepted unowned staging evidence")
			}
		})
	}
}

func TestPolicyWritesUseNativePublicationOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy")
	owner := PolicyOwner{Product: "starmap", Deployment: "deployment", Instance: "instance"}
	_, err := OpenFilePolicyStore(t.Context(), path, owner, EnvironmentPolicyCurrent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, privatefiles.PublicationDirectoryName, ".owner.lock")); err != nil {
		t.Fatal(err)
	}
	if err := InspectFilePolicyStore(t.Context(), path, owner, EnvironmentPolicyCurrent); err != nil {
		t.Fatal(err)
	}
}
