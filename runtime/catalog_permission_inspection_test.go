package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
)

func retainedCatalogPermissionFixture(t *testing.T) (RetainedCatalogPermissionRequest, *layerStore, authorityPermissions, permission.ClockReading) {
	t.Helper()
	directory, owner, store := retainedDirectoryFixture(t, "permission-inspection")
	state := retainedAuthorityPermissions(t)
	if err := store.savePermission(t.Context(), state, false); err != nil {
		t.Fatal(err)
	}
	policy := DefaultSourcePolicy()
	policy.Kind, policy.URL, policy.StartupPolicy = SourceStarmap, "http://127.0.0.1:1", StartupRequireAuthority
	policy.AuthorityID, policy.PolicyID = state.authorityID, state.policyID
	request := RetainedCatalogPermissionRequest{Directory: directory, Owner: owner, SchedulerIdentity: "permission-inspection", SourcePolicy: policy, AcceptedHead: state.enforced}
	return request, store, state, permission.ClockReading{Time: state.required.IssuedAt.Add(time.Second), Known: true}
}

func TestRetainedCatalogPermissionExpiredRefuses(t *testing.T) {
	request, _, state, clock := retainedCatalogPermissionFixture(t)
	clock.Time = state.required.ValidUntil
	if _, err := InspectRetainedCatalogPermission(t.Context(), request, clock); err == nil {
		t.Fatal("expired authority checkpoint passed admission inspection")
	}
}

func TestRetainedCatalogPermissionPreservesOriginalEvidence(t *testing.T) {
	request, _, state, clock := retainedCatalogPermissionFixture(t)
	before := retainedDirectoryHashes(t, request.Directory)
	checked, err := InspectRetainedCatalogPermission(t.Context(), request, clock)
	if err != nil {
		t.Fatal(err)
	}
	original := checked.Record()
	if checked.Digest() == "" || len(original) == 0 || len(original) > maxCatalogPermissionRequestBytes {
		t.Fatal("missing or unbounded original evidence")
	}
	copy := checked.Record()
	copy[0] ^= 1
	if !bytes.Equal(original, checked.Record()) {
		t.Fatal("caller changed sealed evidence")
	}
	clock.Time = clock.Time.Add(time.Minute)
	if err := checked.Check(t.Context(), request, clock); err != nil {
		t.Fatal(err)
	}
	reopened, err := InspectRetainedCatalogPermission(t.Context(), request, clock)
	if err != nil || reopened.Digest() != checked.Digest() || !bytes.Equal(original, reopened.Record()) {
		t.Fatal("passive reopen changed the original evidence", err)
	}
	clock.Time = state.required.ValidUntil
	if err := checked.Check(t.Context(), request, clock); err == nil {
		t.Fatal("reinspection extended receipt expiry")
	}
	if after := retainedDirectoryHashes(t, request.Directory); !reflect.DeepEqual(before, after) {
		t.Fatal("passive checks changed retained files")
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if got := fmt.Sprintf(format, checked); strings.Contains(got, "request_sha256") || strings.Contains(got, request.Directory) || strings.Contains(got, state.required.Head.AuthorityID) {
			t.Fatal("diagnostics exposed original native evidence")
		}
	}
}

func TestRetainedCatalogPermissionRefusesInvalidAuthorityEvidence(t *testing.T) {
	for _, name := range []string{"unknown clock", "uncertainty", "before issue", "uncertain checkpoint", "missing checkpoint", "corrupt checkpoint", "oversize checkpoint", "wrong policy", "wrong authority", "wrong selected head", "known withdrawal", "omitted authority", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			request, store, state, clock := retainedCatalogPermissionFixture(t)
			ctx := t.Context()
			switch name {
			case "unknown clock":
				clock.Known = false
			case "uncertainty":
				clock.Uncertainty = state.required.ValidUntil.Sub(clock.Time)
			case "before issue":
				clock.Time = state.required.IssuedAt.Add(-time.Second)
			case "uncertain checkpoint":
				if err := store.savePermission(ctx, state, true); err != nil {
					t.Fatal(err)
				}
			case "missing checkpoint":
				if err := os.Remove(filepath.Join(request.Directory, layerDirectoryName, permissionCheckpointFile)); err != nil {
					t.Fatal(err)
				}
			case "corrupt checkpoint", "oversize checkpoint":
				data := []byte("{broken")
				if name == "oversize checkpoint" {
					data = bytes.Repeat([]byte("x"), maxPermissionCheckpointBytes+1)
				}
				if err := os.WriteFile(filepath.Join(request.Directory, layerDirectoryName, permissionCheckpointFile), data, 0o600); err != nil {
					t.Fatal(err)
				}
			case "wrong policy":
				request.SourcePolicy.PolicyID = "other"
			case "wrong authority":
				request.SourcePolicy.AuthorityID = "other"
			case "wrong selected head":
				request.AcceptedHead.GenerationID = "other"
			case "known withdrawal":
				next := state.highest
				next.Sequence++
				next.GenerationID = "withdrawn"
				next.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
				var err error
				state, err = state.observeHead(next)
				if err != nil {
					t.Fatal(err)
				}
				if err := store.savePermission(ctx, state, false); err != nil {
					t.Fatal(err)
				}
			case "omitted authority":
				request.SourcePolicy = DefaultSourcePolicy()
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			before := retainedDirectoryHashes(t, request.Directory)
			if checked, err := InspectRetainedCatalogPermission(ctx, request, clock); err == nil || checked != nil {
				t.Fatal("invalid authority evidence produced permission")
			}
			if after := retainedDirectoryHashes(t, request.Directory); !reflect.DeepEqual(before, after) {
				t.Fatal("refusal changed retained evidence")
			}
		})
	}
}

func TestRetainedCatalogPermissionRefusesChangedOriginalIdentity(t *testing.T) {
	for _, name := range []string{"seed", "owner", "scheduler", "source", "renewal", "checkpoint identity", "directory identity", "layer identity"} {
		t.Run(name, func(t *testing.T) {
			request, store, state, clock := retainedCatalogPermissionFixture(t)
			checked, err := InspectRetainedCatalogPermission(t.Context(), request, clock)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "seed":
				if err := os.WriteFile(filepath.Join(request.Directory, instanceSeedFileName), []byte(strings.Repeat("a", instanceSeedBytes*2)), 0o600); err != nil {
					t.Fatal(err)
				}
			case "owner":
				request.Owner.Instance = "replica-b"
			case "scheduler":
				request.SchedulerIdentity = "another-scheduler"
			case "source":
				request.SourcePolicy.URL = "http://127.0.0.1:2"
			case "renewal":
				renewal := state.required
				renewal.IssuedAt = renewal.IssuedAt.Add(time.Second)
				renewal.ValidUntil = renewal.ValidUntil.Add(time.Second)
				state, err = state.observe(renewal)
				if err == nil {
					state, err = state.confirmRetention(renewal)
				}
				if err == nil {
					err = store.savePermission(t.Context(), state, false)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "checkpoint identity":
				name := filepath.Join(request.Directory, layerDirectoryName, permissionCheckpointFile)
				body, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(name, name+".old"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, body, 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory identity", "layer identity":
				target := request.Directory
				if name == "layer identity" {
					target = filepath.Join(target, layerDirectoryName)
				}
				if err := os.Rename(target, target+".old"); err != nil {
					t.Fatal(err)
				}
				if err := copyPermissionFixtureTree(target+".old", target); err != nil {
					t.Fatal(err)
				}
				if _, err := InspectRetainedCatalogPermission(t.Context(), request, clock); err != nil {
					t.Fatal("replacement fixture is structurally invalid", err)
				}
			}
			if err := checked.Check(t.Context(), request, clock); err == nil {
				t.Fatal("changed original identity passed reinspection")
			}
		})
	}
}

func copyPermissionFixtureTree(source, destination string) error {
	return filepath.Walk(source, func(name string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return os.Mkdir(target, 0o700)
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			return err
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	})
}

func TestRetainedCatalogPermissionOrdinaryCatalogNeedsNoQualifiedClock(t *testing.T) {
	directory, owner, store := retainedDirectoryFixture(t, "ordinary-inspection")
	request := RetainedCatalogPermissionRequest{Directory: directory, Owner: owner, SchedulerIdentity: "ordinary-inspection", SourcePolicy: DefaultSourcePolicy()}
	before := retainedDirectoryHashes(t, directory)
	checked, err := InspectRetainedCatalogPermission(t.Context(), request, permission.ClockReading{})
	if err != nil {
		t.Fatal(err)
	}
	if err := checked.Check(t.Context(), request, permission.ClockReading{}); err != nil {
		t.Fatal(err)
	}
	if after := retainedDirectoryHashes(t, directory); !reflect.DeepEqual(before, after) {
		t.Fatal("ordinary inspection changed files")
	}
	if err := store.savePermission(t.Context(), retainedAuthorityPermissions(t), true); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectRetainedCatalogPermission(t.Context(), request, permission.ClockReading{}); err == nil {
		t.Fatal("ordinary policy discarded retained authority requirements")
	}
	var absent *RetainedCatalogPermission
	if absent.Record() != nil || absent.Digest() != "" || absent.Check(t.Context(), request, permission.ClockReading{}) == nil {
		t.Fatal("absent capability passed inspection")
	}
}

func TestRetainedCatalogPermissionRefusesInvalidRequestAndPendingPublication(t *testing.T) {
	for _, kind := range []string{"nil context", "relative directory", "noncanonical directory", "missing directory", "owner record", "unbounded source", "invalid source", "pending root", "pending layers"} {
		t.Run(kind, func(t *testing.T) {
			request, _, _, clock := retainedCatalogPermissionFixture(t)
			originalDirectory := request.Directory
			ctx := t.Context()
			switch kind {
			case "nil context":
				ctx = nil
			case "relative directory":
				request.Directory = "relative"
			case "noncanonical directory":
				request.Directory += "/."
			case "missing directory":
				request.Directory = filepath.Join(request.Directory, "missing")
			case "owner record":
				if err := os.WriteFile(filepath.Join(request.Directory, ownerRecordName), []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unbounded source":
				request.SourcePolicy.URL = strings.Repeat("x", maxCatalogPermissionRequestBytes+1)
			case "invalid source":
				request.SourcePolicy.MaxHops = 0
			case "pending root", "pending layers":
				directory := request.Directory
				if kind == "pending layers" {
					directory = filepath.Join(directory, layerDirectoryName)
				}
				metadata := filepath.Join(directory, privatefiles.PublicationDirectoryName)
				if err := os.MkdirAll(metadata, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(metadata, strings.Repeat("A", 26)+".jsonl"), []byte("unresolved"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := retainedDirectoryHashes(t, originalDirectory)
			if checked, err := InspectRetainedCatalogPermission(ctx, request, clock); err == nil || checked != nil {
				t.Fatal("invalid request produced retained permission")
			}
			if after := retainedDirectoryHashes(t, originalDirectory); !reflect.DeepEqual(before, after) {
				t.Fatal("refusal changed files")
			}
		})
	}
}
