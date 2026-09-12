package storage

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestFilesystemRecordReadsRejectOversize(t *testing.T) {
	for _, record := range []struct {
		name  string
		limit int64
	}{
		{currentFilename, catalogs.MaxCatalogAuthorityRecordBytes},
		{manifestFilename, 64 << 20},
		{payloadFilename, resourcepolicy.MaxPayloadBytes},
	} {
		t.Run(record.name, func(t *testing.T) {
			root := privateFilesystemRoot(t)
			if err := privatefiles.CreateDirectory(root); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, record.name)
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, privatefiles.FileMode)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(record.limit + 1); err != nil {
				_ = file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := readPrivateStoreFile(path)
			var validation *errors.ValidationError
			if !stderrors.As(err, &validation) || len(data) != 0 {
				t.Fatalf("oversized record entered memory: bytes=%d, error=%v", len(data), err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() != record.limit+1 {
				t.Fatalf("size refusal changed the file: %v", err)
			}
		})
	}
}

func TestFilesystemRejectsUnreadableCandidate(t *testing.T) {
	for _, record := range []string{"pointer", "payload", "manifest"} {
		t.Run(record, func(t *testing.T) {
			root := privateFilesystemRoot(t)
			store, err := NewFilesystem(root)
			if err != nil {
				t.Fatal(err)
			}
			generation := testGeneration("oversized-record", "value")
			switch record {
			case "pointer":
				generation.Manifest.GenerationID = strings.Repeat("x", catalogs.MaxCatalogAuthorityRecordBytes)
			case "payload":
				generation.Payload = make([]byte, resourcepolicy.MaxPayloadBytes+1)
				generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
			case "manifest":
				generation.Manifest.SyncRunID = strings.Repeat("x", 64<<20)
			}
			err = store.Commit(t.Context(), generation, "")
			var validation *errors.ValidationError
			if !stderrors.As(err, &validation) {
				t.Fatalf("committed a candidate beyond the %s read limit: %v", record, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "current")); !stderrors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid candidate published current: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, "generations")); !stderrors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid candidate created generation state: %v", err)
			}
		})
	}
}

func TestFilesystemPointerLimitRoundTripAndRefusalPreservesCurrent(t *testing.T) {
	root := privateFilesystemRoot(t)
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	generation := testGeneration("pointer-boundary", "value")
	generation.Manifest.GenerationID = strings.Repeat("x", catalogs.MaxCatalogAuthorityRecordBytes-1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatalf("commit at the exact pointer limit: %v", err)
	}
	got, err := store.Current(t.Context())
	if err != nil || !sameGeneration(generation, got) {
		t.Fatalf("restore at the exact pointer limit: %v", err)
	}
	oversized := generation.Copy()
	oversized.Manifest.GenerationID += "x"
	err = store.Commit(t.Context(), oversized, generation.Manifest.GenerationID)
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) {
		t.Fatalf("accepted oversized replacement: %v", err)
	}
	got, err = store.Current(t.Context())
	if err != nil || !sameGeneration(generation, got) {
		t.Fatalf("refused replacement changed current: %v", err)
	}
	if err := store.Commit(t.Context(), generation, generation.Manifest.GenerationID); err != nil {
		t.Fatalf("idempotent retry at the exact limit: %v", err)
	}
}
