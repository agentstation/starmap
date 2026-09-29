package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func retainedBaselineFixture(t *testing.T) (string, string, catalogs.Generation) {
	t.Helper()
	generation, err := Generation()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalogs.NewEmpty())
	if err != nil {
		t.Fatal(err)
	}
	generation.Payload = payload
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	generation.Manifest.GenerationID = "retained-before-binary-upgrade"
	digest := sha256.Sum256([]byte(generation.Manifest.GenerationID))
	root := filepath.Join(t.TempDir(), "exports")
	path := filepath.Join(root, hex.EncodeToString(digest[:]))
	if err := privatefiles.CreateDirectory(path); err != nil {
		t.Fatal(err)
	}
	writeRetainedBaseline(t, path, generation)
	return root, path, generation
}

func writeRetainedBaseline(t *testing.T, path string, generation catalogs.Generation) {
	t.Helper()
	manifest, err := json.MarshalIndent(generation.Manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{baselineManifestName: append(manifest, '\n'), baselinePayloadName: generation.Payload} {
		if err := os.WriteFile(filepath.Join(path, name), body, baselineFileMode); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInspectBaselineExportsRetainsOlderCompatibleGeneration(t *testing.T) {
	root, path, generation := retainedBaselineFixture(t)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := InspectExports(t.Context(), root); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("inspection replaced the export")
	}
	body, err := os.ReadFile(filepath.Join(path, baselinePayloadName))
	if err != nil || string(body) != string(generation.Payload) {
		t.Fatal("inspection changed the payload")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("inspection created recovery state")
	}
}

func TestInspectBaselineExportsRefusesInvalidStateWithoutRepair(t *testing.T) {
	for _, mode := range []string{"checksum", "semantic-payload", "schema-mismatch", "missing-payload", "extra-file", "nested-directory", "wrong-name", "uppercase-name", "unknown-manifest-field", "oversize-manifest", "journal", "unfinished-stage", "empty", "nil-context", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			root, path, generation := retainedBaselineFixture(t)
			ctx := t.Context()
			write := func(name string, body []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(path, name), body, baselineFileMode); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "checksum":
				write(baselinePayloadName, []byte("changed"))
			case "semantic-payload":
				generation.Payload = []byte(`{"schema_version":19}`)
				generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
				writeRetainedBaseline(t, path, generation)
			case "schema-mismatch":
				generation.Manifest.SchemaVersion = catalogs.CurrentCatalogSchemaVersion + 1
				generation.Manifest.ConsumerCompatibility.MaxSchemaVersion = generation.Manifest.SchemaVersion
				writeRetainedBaseline(t, path, generation)
			case "missing-payload":
				if err := os.Remove(filepath.Join(path, baselinePayloadName)); err != nil {
					t.Fatal(err)
				}
			case "extra-file":
				write("unexpected", []byte("preserve"))
			case "nested-directory":
				if err := privatefiles.CreateDirectory(filepath.Join(path, "unexpected")); err != nil {
					t.Fatal(err)
				}
			case "wrong-name", "uppercase-name":
				name := strings.Repeat("a", 64)
				if mode == "uppercase-name" {
					name = strings.ToUpper(filepath.Base(path))
				}
				if err := os.Rename(path, filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			case "unknown-manifest-field":
				body, err := json.Marshal(generation.Manifest)
				if err != nil {
					t.Fatal(err)
				}
				write(baselineManifestName, append([]byte(`{"unknown":true,`), body[1:]...))
			case "oversize-manifest":
				file, err := os.OpenFile(filepath.Join(path, baselineManifestName), os.O_WRONLY, baselineFileMode)
				if err != nil {
					t.Fatal(err)
				}
				err = file.Truncate(catalogs.MaxCatalogPayloadBytes + 1)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatal(err, closeErr)
				}
			case "journal", "unfinished-stage":
				name := baselineRecoveryDirectory
				if mode == "unfinished-stage" {
					name = baselineStagePrefix + "incomplete"
				}
				if err := privatefiles.CreateDirectory(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			case "empty":
				root = filepath.Join(t.TempDir(), "empty")
				if err := privatefiles.CreateDirectory(root); err != nil {
					t.Fatal(err)
				}
			case "nil-context":
				ctx = nil
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			before, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := InspectExports(ctx, root); err == nil {
				t.Fatal("invalid export accepted")
			}
			after, err := os.ReadDir(root)
			if err != nil || len(before) != len(after) {
				t.Fatal("inspection changed the tree")
			}
		})
	}
}

func TestInspectBaselineExportsDoesNotCreateMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	if err := InspectExports(t.Context(), path); !os.IsNotExist(err) {
		t.Fatalf("got %v, want missing directory", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("inspection created a directory")
	}
}
