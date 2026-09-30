package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func comparisonFixture(t *testing.T) (CatalogRetentionRequest, CatalogRetentionReceipt, CatalogRetainedReadRequest, *CatalogRetentionComparison) {
	t.Helper()
	request, _ := retentionFixture(t, 2)
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := InspectCatalogRetentionComparison(t.Context(), request.Manifest[0], request.Inputs[0])
	if err != nil {
		t.Fatal(err)
	}
	read := CatalogRetainedReadRequest{Directory: request.Directory, Owner: request.Owner, SchedulerIdentity: request.SchedulerIdentity, TransferID: request.TransferID, Batch: retentionBatchReference(t, receipt), Index: 0}
	return request, receipt, read, proof
}

func comparisonTree(t *testing.T, directory string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash := sha256.Sum256(data)
			value += ":" + hex.EncodeToString(hash[:])
		}
		files[path] = value
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestCatalogRetentionComparisonChecksOriginalBytesWithoutMutation(t *testing.T) {
	request, _, read, proof := comparisonFixture(t)
	before := comparisonTree(t, request.Directory)
	usage, err := InspectCatalogRetentionUsage(t.Context(), request.Manifest[0], request.Inputs[0])
	if err != nil || proof.Usage() != usage {
		t.Fatal("comparison did not retain original checked usage", err)
	}
	request.Inputs[0].Recovery.Inputs.Data[0] ^= 1
	request.Inputs[0].Generation.Payload[0] ^= 1
	request.Manifest[0].ManifestSHA256 = strings.Repeat("a", 64)
	if err := CheckRetainedCatalogRecovery(t.Context(), read, proof); err != nil {
		t.Fatal("caller mutation changed original comparison", err)
	}
	if after := comparisonTree(t, request.Directory); !reflect.DeepEqual(before, after) {
		t.Fatal("passive comparison changed native files or access modes")
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		for _, candidate := range []any{proof, *proof} {
			if fmt.Sprintf(verb, candidate) != "CatalogRetentionComparison{private}" {
				t.Fatal("comparison diagnostics disclosed private evidence")
			}
		}
		if fmt.Sprintf(verb, (*CatalogRetentionComparison)(nil)) != "<nil>" {
			t.Fatal("nil comparison diagnostics failed")
		}
	}
	if _, err := json.Marshal(proof); err == nil {
		t.Fatal("private comparison encoded a serialized constructor")
	}
	var detached CatalogRetentionComparison
	if err := json.Unmarshal([]byte(`{}`), &detached); err == nil {
		if err := CheckRetainedCatalogRecovery(t.Context(), read, &detached); err == nil {
			t.Fatal("serialized comparison granted original evidence")
		}
	}
	if (*CatalogRetentionComparison)(nil).Usage().Inputs() != 0 || (new(CatalogRetentionComparison)).Usage().Inputs() != 0 {
		t.Fatal("empty comparison reported checked input")
	}
}

func TestCatalogRetentionComparisonRefusesChangedNativeEvidence(t *testing.T) {
	for _, field := range []string{"nil", "zero", "foreign", "owner", "scheduler", "transfer", "batch", "receipt", "index", "manifest", "seed", "envelope", "truncated", "extended", "missing", "incomplete", "pending"} {
		t.Run(field, func(t *testing.T) {
			request, receipt, read, proof := comparisonFixture(t)
			transfer := filepath.Join(request.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(request.TransferID)))
			envelope := filepath.Join(transfer, receipt.Records[0].RecordSHA256+".json.gz")
			journal := filepath.Join(request.Directory, layerDirectoryName, materializationsDirectory, fleetRecoveryChecksum([]byte(request.OperationID)))
			write := func(path string, data []byte) {
				t.Helper()
				if err := os.WriteFile(path, data, ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			}
			readFile := func(path string) []byte {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			switch field {
			case "nil":
				proof = nil
			case "zero":
				proof = &CatalogRetentionComparison{}
			case "foreign":
				var err error
				proof, err = InspectCatalogRetentionComparison(t.Context(), request.Manifest[1], request.Inputs[1])
				if err != nil {
					t.Fatal(err)
				}
			case "owner":
				read.Owner.Deployment += "-changed"
			case "scheduler":
				read.SchedulerIdentity += "-changed"
			case "transfer":
				read.TransferID += "-changed"
			case "batch":
				read.Batch.OperationID += "-changed"
			case "receipt":
				read.Batch.ReceiptSHA256 = strings.Repeat("a", 64)
			case "index":
				read.Index = 1
			case "manifest":
				manifest, err := catalogRetentionManifestBytes(request.TransferID, []CatalogRetentionEntry{request.Manifest[1], request.Manifest[0]})
				if err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(transfer, "manifest.json"), manifest)
			case "seed":
				path := filepath.Join(request.Directory, instanceSeedFileName)
				seed := readFile(path)
				replacement := bytes.Repeat([]byte("a"), len(seed))
				if bytes.Equal(seed, replacement) {
					replacement = bytes.Repeat([]byte("b"), len(seed))
				}
				owner, err := privatefiles.ExistingDirectory(request.Directory)
				if err != nil {
					t.Fatal(err)
				}
				if err := owner.WriteFile(instanceSeedFileName, replacement, ".fixture-"); err != nil {
					t.Fatal(err)
				}
				if err := inspectMaterializationOwner(t.Context(), owner, materializationOwnerRequest(request)); err != nil {
					t.Fatal("replacement seed was not valid current owner state", err)
				}
			case "envelope":
				data := readFile(envelope)
				data[len(data)/2] ^= 1
				write(envelope, data)
			case "truncated":
				data := readFile(envelope)
				write(envelope, data[:len(data)-1])
			case "extended":
				write(envelope, append(readFile(envelope), 0))
			case "missing":
				if err := os.Remove(envelope); err != nil {
					t.Fatal(err)
				}
			case "incomplete":
				if err := os.Remove(filepath.Join(journal, materializationReceiptName)); err != nil {
					t.Fatal(err)
				}
			case "pending":
				directory, err := privatefiles.ExistingDirectory(journal)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := directory.Child(".record-publications"); err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(journal, ".record-publications", "foreign-stage.jsonl"), []byte("preserve pending evidence"))
			}
			before := comparisonTree(t, request.Directory)
			if err := CheckRetainedCatalogRecovery(t.Context(), read, proof); err == nil {
				t.Fatal("changed original or native evidence passed")
			}
			if after := comparisonTree(t, request.Directory); !reflect.DeepEqual(before, after) {
				t.Fatal("refusal repaired or removed native evidence")
			}
		})
	}
}

func TestCatalogRetentionComparisonRequiresCompleteOriginalValidation(t *testing.T) {
	request, _ := retentionFixture(t, 1)
	for _, field := range []string{"entry", "origin", "descriptor", "payload", "recovery"} {
		t.Run(field, func(t *testing.T) {
			entry, input := request.Manifest[0], request.Inputs[0]
			switch field {
			case "entry":
				entry.ManifestSHA256 = strings.Repeat("a", 64)
			case "origin":
				entry.SourceOrigin = "unknown"
			case "descriptor":
				input.SourceDescriptor = []byte("unbound source")
			case "payload":
				input.Generation = input.Generation.Copy()
				input.Generation.Payload[0] ^= 1
			case "recovery":
				input.Recovery.Inputs.Data = []byte("invalid compressed original")
				input.Recovery.Inputs.Checksum = fleetRecoveryChecksum(input.Recovery.Inputs.Data)
				entry.InputsSHA256 = input.Recovery.Inputs.Checksum
			}
			if proof, err := InspectCatalogRetentionComparison(t.Context(), entry, input); err == nil || proof != nil {
				t.Fatal("invalid original produced comparison evidence", err)
			}
		})
	}
	if proof, err := InspectCatalogRetentionComparison(nil, request.Manifest[0], request.Inputs[0]); err == nil || proof != nil {
		t.Fatal("nil context produced comparison evidence", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if proof, err := InspectCatalogRetentionComparison(ctx, request.Manifest[0], request.Inputs[0]); !errors.Is(err, context.Canceled) || proof != nil {
		t.Fatal("cancellation produced comparison evidence", err)
	}
	_, _, read, proof := comparisonFixture(t)
	if err := CheckRetainedCatalogRecovery(ctx, read, proof); !errors.Is(err, context.Canceled) {
		t.Fatal("native comparison ignored cancellation", err)
	}
	if err := CheckRetainedCatalogRecovery(nil, read, proof); err == nil {
		t.Fatal("native comparison accepted nil context")
	}
}
