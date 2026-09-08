//go:build darwin || linux

package app

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/runtime"
)

func TestFileRolePolicyMatchesRuntimeEnforcement(t *testing.T) {
	for _, role := range []string{"configuration", "dotenv", "catalog-store", "runtime-lock", "workspace"} {
		t.Run(role, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0o700); err != nil {
				t.Fatal(err)
			}
			target := root
			kind := "directory"
			if role == "configuration" || role == "dotenv" || role == "runtime-lock" {
				name := "config"
				if role == "runtime-lock" {
					name = ".owner.lock"
				}
				target = filepath.Join(root, name)
				kind = "file"
				if err := os.WriteFile(target, []byte("operator input"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			declared, err := managedFilePolicy(role)
			if role == "dotenv" {
				declared, err = externalFilePolicy(role)
			}
			if err != nil {
				t.Fatal(err)
			}
			access := func() error {
				switch role {
				case "configuration", "dotenv":
					_, err := readPrivateInput(target, role)
					return err
				case "catalog-store":
					store, err := storage.NewFilesystem(root)
					if err != nil {
						return err
					}
					_, err = store.Current(t.Context())
					var absent *errors.NotFoundError
					if stderrors.As(err, &absent) {
						return nil
					}
					return err
				case "runtime-lock":
					return runtime.ValidateDirectoryPermissions(t.Context(), root)
				default:
					return workspace.Read(t.Context(), root, func(input workspace.InputExpectation) error {
						if !input.Exists {
							t.Fatal("workspace read lost the existing input")
						}
						return nil
					})
				}
			}
			if err := access(); err != nil {
				t.Fatal("private input refused", err)
			}
			mode := os.FileMode(0o640)
			if kind == "directory" {
				mode = 0o770
			}
			if err := os.Chmod(target, mode); err != nil {
				t.Fatal(err)
			}
			manifest := productpaths.FileManifest{Files: []productpaths.FileEntry{{ID: role, Location: productpaths.Path{Path: target}, Kind: "file", Availability: "available", Policy: declared}}}
			report, err := productpaths.InspectManifest(t.Context(), manifest, 10)
			if err != nil || len(report.Observations) != 1 {
				t.Fatal("inspection", report, err)
			}
			item := report.Observations[0]
			err = access()
			if role == "workspace" {
				if err != nil || item.AccessPolicy != "deployment-controlled" || item.AccessStatus != "unverified" {
					t.Fatal("workspace gained private-store restrictions", item, err)
				}
			} else {
				var invalid *errors.ValidationError
				if !stderrors.As(err, &invalid) || item.AccessPolicy != "owner-only" || item.AccessStatus != "conflict" || item.AccessReason != "group-or-other-mode-bits" {
					t.Fatal("runtime and inspection disagree", item, err)
				}
			}
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatal("access checks changed permissions", err)
			}
			if kind == "file" {
				data, err := os.ReadFile(target)
				if err != nil || string(data) != "operator input" {
					t.Fatal("access checks changed input bytes", err)
				}
			}
		})
	}
}

func TestPrivateInputRefusesIncompatibleRoleBeforeFilesystemAccess(t *testing.T) {
	root := t.TempDir()
	for _, role := range []string{"workspace", "baseline", "unknown"} {
		_, err := readPrivateInput(filepath.Join(root, "absent"), role)
		var invalid *errors.ConfigError
		if !stderrors.As(err, &invalid) || invalid.Component != "file policy" {
			t.Fatalf("role %s reached the filesystem: %v", role, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("role refusal created files", err)
	}
}
