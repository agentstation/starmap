package main

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestArtifactReleaseCommandStagesExactPromotion(t *testing.T) {
	_, releasePath, generation := promotionFixture(t)
	target := filepath.Join(t.TempDir(), "promotion")
	args := []string{"--stage-promotion-dir", target, "--promotion-release-dir", releasePath}
	var output bytes.Buffer
	if err := run(args, &output); err != nil {
		t.Fatalf("stage exact promotion: %v", err)
	}
	var report promotionReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.GenerationID != generation.Manifest.GenerationID || report.PayloadChecksum != generation.Manifest.Payload.Checksum {
		t.Fatalf("staging changed release identity: %+v", report)
	}
	verified, err := verifyPromotionDirectory(target, releasePath)
	if err != nil || verified != report {
		t.Fatalf("staged catalog differs from the release: %+v, %v", verified, err)
	}
	manifestPath := filepath.Join(target, "generation.json")
	before, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var retry bytes.Buffer
	if err := run(args, &retry); err != nil {
		t.Fatalf("reuse exact staging: %v", err)
	}
	after, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || !bytes.Equal(output.Bytes(), retry.Bytes()) {
		t.Fatal("retry rewrote complete staging or changed its report")
	}
}

func TestArtifactReleaseCommandPreservesRejectedPromotionStage(t *testing.T) {
	for _, kind := range []string{"existing", "modified", "invalid_release"} {
		t.Run(kind, func(t *testing.T) {
			_, releasePath, _ := promotionFixture(t)
			target := filepath.Join(t.TempDir(), "promotion")
			args := []string{"--stage-promotion-dir", target, "--promotion-release-dir", releasePath}
			preservedPath := filepath.Join(target, "operator.txt")
			preserved := []byte("operator data\n")
			switch kind {
			case "existing":
				if err := os.Mkdir(target, constants.DirPermissions); err != nil {
					t.Fatal(err)
				}
				writePromotionFile(t, preservedPath, preserved)
			case "modified":
				var first bytes.Buffer
				if err := run(args, &first); err != nil {
					t.Fatal(err)
				}
				preservedPath = filepath.Join(target, "generation.json")
				writePromotionFile(t, preservedPath, preserved)
			case "invalid_release":
				writePromotionFile(t, filepath.Join(releasePath, artifact.Filename), []byte("invalid archive"))
			}
			var output bytes.Buffer
			if err := run(args, &output); err == nil || output.Len() != 0 {
				t.Fatalf("rejected stage reported success: %v, %s", err, output.String())
			}
			if kind == "invalid_release" {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatalf("invalid release created a destination: %v", err)
				}
				return
			}
			data, err := os.ReadFile(preservedPath)
			if err != nil || !bytes.Equal(data, preserved) {
				t.Fatalf("rejected stage changed existing data: %v", err)
			}
		})
	}
}

func TestPromotionStageRetryConfirmsDirectorySync(t *testing.T) {
	_, releasePath, generation := promotionFixture(t)
	target := filepath.Join(t.TempDir(), "promotion")
	calls := 0
	stager := promotionStager{syncDirectory: func(*os.Root) error {
		calls++
		return fs.ErrPermission
	}}
	var publication *errors.PublicationError
	if _, err := stager.stage(target, releasePath); !stderrors.As(err, &publication) {
		t.Fatalf("missing ambiguous publication result: %v", err)
	}
	if _, err := verifyPromotionDirectory(target, releasePath); err != nil {
		t.Fatalf("failure did not preserve the complete visible catalog: %v", err)
	}
	calls = 0
	if _, err := stager.stage(target, releasePath); !stderrors.As(err, &publication) || calls != 1 {
		t.Fatalf("retry did not confirm directory durability: calls=%d, error=%v", calls, err)
	}
	stager.syncDirectory = func(root *os.Root) error {
		calls++
		return filepublish.SyncDirectory(root)
	}
	calls = 0
	report, err := stager.stage(target, releasePath)
	if err != nil || calls != 1 || report.GenerationID != generation.Manifest.GenerationID {
		t.Fatalf("durable retry changed the catalog or missed the flush: %+v, %d, %v", report, calls, err)
	}
}
