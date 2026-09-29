package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	githubsource "github.com/agentstation/starmap/internal/sources/github"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func retainedDirectoryFixture(t *testing.T, identity string) (string, DirectoryOwner, *layerStore) {
	t.Helper()
	directory := privateRuntimeDirectory(t)
	owner := DirectoryOwner{Product: "starport", Deployment: "recovery-deployment", Instance: "replica-a"}
	lock, err := acquireDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := bindDirectoryOwner(t.Context(), directory, owner, identity); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareInstanceSeed(t.Context(), directory); err != nil {
		t.Fatal(err)
	}
	store, err := newLayerStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	return directory, owner, store
}

func retainedDirectoryHashes(t *testing.T, directory string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(directory, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		relative, err := filepath.Rel(directory, name)
		if err != nil {
			return err
		}
		result[relative] = hex.EncodeToString(digest[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRetainedDirectoryInspectionPreservesIdentityAndInputs(t *testing.T) {
	directory, owner, store := retainedDirectoryFixture(t, "operator-retained-scheduler")
	at := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	layer := scopedProviderLayer(t, "account-a", "1", at)
	if err := store.saveProvider(t.Context(), layer); err != nil {
		t.Fatal(err)
	}
	payload := testCatalogPayload(t, "source-provider", "source-model", "Source")
	source := sourceLayer{Identity: "retained-upstream", GenerationID: "retained-source", Checksum: catalogs.DescribeCatalogPayload(payload).Checksum, Payload: payload, ObservedAt: at}
	if err := store.saveSource(t.Context(), source); err != nil {
		t.Fatal(err)
	}
	observations, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "manual-model", at, false)})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := store.stageManualBatch(t.Context(), &manualBatch{observations: observations})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.saveManualHead(t.Context(), batch); err != nil {
		t.Fatal(err)
	}
	pending, err := store.stageInput(t.Context(), layer)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.writeInputPublication(t.Context(), inputPublication{Version: inputPublicationVersion, Phase: inputPublicationPrepared,
		ExpectedID: "before", ExpectedChecksum: "sha256:" + strings.Repeat("a", 64), GenerationID: "after", PayloadChecksum: "sha256:" + strings.Repeat("b", 64), Providers: []string{pending}, Manual: batch}); err != nil {
		t.Fatal(err)
	}
	permission := authorityPermissionFixture()
	p, err := (authorityPermissions{authorityID: permission.Head.AuthorityID, policyID: permission.Head.PolicyID}).observe(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.savePermission(t.Context(), p, true); err != nil {
		t.Fatal(err)
	}
	before := retainedDirectoryHashes(t, directory)
	if err := InspectRetainedDirectory(t.Context(), directory, owner, "operator-retained-scheduler"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, retainedDirectoryHashes(t, directory)) {
		t.Fatal("inspection changed retained evidence")
	}
	publication, err := store.loadInputPublication()
	if err != nil || publication == nil || publication.Phase != inputPublicationPrepared {
		t.Fatal("inspection resolved a prepared transaction")
	}
	restored, err := store.loadPermission(authorityPermissions{authorityID: permission.Head.AuthorityID, policyID: permission.Head.PolicyID})
	if err != nil || restored.retained || restored.pending {
		t.Fatal("inspection confirmed uncertain permission")
	}
}

func TestRetainedDirectoryInspectionRefusesIncompleteOrConflictingState(t *testing.T) {
	for _, mode := range []string{"owner", "override", "seed", "missing-seed", "missing-owner", "unknown-file", "unknown-directory", "migration-receipt", "permission", "pin", "manual", "source", "provider", "input-hash", "publication", "pending-record", "canceled", "nil"} {
		t.Run(mode, func(t *testing.T) {
			directory, owner, store := retainedDirectoryFixture(t, "")
			ctx := t.Context()
			identity := ""
			write := func(name string, body []byte) {
				t.Helper()
				if err := privatefiles.CreateDirectory(filepath.Dir(filepath.Join(directory, name))); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "owner":
				owner.Instance = "replica-b"
			case "override":
				identity = "different-scheduler"
			case "seed":
				write(instanceSeedFileName, []byte(strings.Repeat("X", 32)))
			case "missing-seed":
				if err := os.Remove(filepath.Join(directory, instanceSeedFileName)); err != nil {
					t.Fatal(err)
				}
			case "missing-owner":
				if err := os.Remove(filepath.Join(directory, ownerRecordName)); err != nil {
					t.Fatal(err)
				}
			case "unknown-file":
				write("unrecognized", []byte("preserve"))
			case "unknown-directory":
				if err := privatefiles.CreateDirectory(filepath.Join(directory, "unrecognized")); err != nil {
					t.Fatal(err)
				}
			case "migration-receipt":
				write(migrationReceiptName, []byte("{}"))
			case "permission":
				write(filepath.Join(layerDirectoryName, permissionCheckpointFile), []byte(`{"version":1,"authority_id":"enterprise","policy_id":"production","receipt":{}}`))
			case "pin":
				write(filepath.Join(layerDirectoryName, generationPinRecordFile), []byte(`{"version":99}`))
			case "manual":
				write(filepath.Join(layerDirectoryName, manualHistoryName), []byte(`{"version":4,"batch":"missing"}`))
			case "source":
				body, err := json.Marshal(sourceLayer{Identity: "upstream", GenerationID: "retained", Checksum: "sha256:" + strings.Repeat("a", 64), Payload: []byte("bad")})
				if err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(layerDirectoryName, sourceLayerFileName), body)
			case "provider":
				write(filepath.Join(layerDirectoryName, providerLayerDirectoryName, "provider.json"), []byte(`{"provider_id":"provider"}`))
			case "input-hash":
				write(filepath.Join(layerDirectoryName, inputPublicationDirectory, strings.Repeat("a", 64)+".json"), []byte(`{}`))
			case "publication":
				if err := store.writeInputPublication(t.Context(), inputPublication{Version: inputPublicationVersion, Phase: inputPublicationCommitted, GenerationID: "after", PayloadChecksum: "checksum", Providers: []string{strings.Repeat("a", 64) + ".json"}}); err != nil {
					t.Fatal(err)
				}
			case "pending-record":
				write(filepath.Join(layerDirectoryName, privatefiles.PublicationDirectoryName, "pending.ndjson"), []byte("pending"))
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "nil":
				ctx = nil
			}
			before := retainedDirectoryHashes(t, directory)
			if err := InspectRetainedDirectory(ctx, directory, owner, identity); err == nil {
				t.Fatal("invalid retained directory accepted")
			}
			if !reflect.DeepEqual(before, retainedDirectoryHashes(t, directory)) {
				t.Fatal("inspection repaired or replaced captured state")
			}
		})
	}
}

func TestRetainedDirectoryInspectionAcceptsClosedRuntime(t *testing.T) {
	directory := privateRuntimeDirectory(t)
	owner := DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"}
	connected := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"))
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	before := retainedDirectoryHashes(t, directory)
	if err := InspectRetainedDirectory(t.Context(), directory, owner, ""); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, retainedDirectoryHashes(t, directory)) {
		t.Fatal("inspection changed closed runtime")
	}
}

func TestRetainedDirectoryInspectionChecksDiscoveryReplayEvidence(t *testing.T) {
	directory, owner, _ := retainedDirectoryFixture(t, "")
	err := privatefiles.CreateDirectory(filepath.Join(directory, "github-catalog-source"))
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	tag, err := artifact.ReleaseTag(digest)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	state := githubsource.State{SchemaVersion: githubsource.StateSchemaVersion, Repository: "agentstation/starmap", Channel: "catalog", Sequence: 42, ChannelChecksum: digest, UpdatedAt: at,
		Verified: githubsource.ReleaseRef{Tag: tag, GenerationID: "retained", CatalogDigest: "sha256:" + digest, VerifiedAt: at}}
	key := sha256.Sum256([]byte(state.Repository + "\x00" + state.Channel))
	name := hex.EncodeToString(key[:]) + ".json"
	for _, invalid := range []bool{false, true} {
		if invalid {
			state.SchemaVersion = 99
		}
		body, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, '\n')
		if err := os.WriteFile(filepath.Join(directory, "github-catalog-source", name), body, 0600); err != nil {
			t.Fatal(err)
		}
		before := retainedDirectoryHashes(t, directory)
		err = InspectRetainedDirectory(t.Context(), directory, owner, "")
		if (err != nil) != invalid {
			t.Fatalf("invalid=%v: inspection error=%v", invalid, err)
		}
		if !reflect.DeepEqual(before, retainedDirectoryHashes(t, directory)) {
			t.Fatal("inspection changed discovery evidence")
		}
	}
}
