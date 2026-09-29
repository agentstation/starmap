package privatefiles

import (
	"context"
	"os"
	"path"
	"strings"
	"testing"
)

func TestRetainedPublicationSelectionRequiresOwnerAndMatchingBytes(t *testing.T) {
	name, raw := retainedJournalFixture(t)
	stage := ".layer-" + strings.TrimSuffix(name, ".jsonl")
	for _, mode := range []string{"valid", "owner-refuses", "nil-owner", "changed-stage", "changed-journal", "missing-lock", "missing-read", "wrong-parent"} {
		t.Run(mode, func(t *testing.T) {
			parent := "scope"
			if mode == "wrong-parent" {
				parent = "other"
			}
			journal := path.Join(parent, publicationDirectory, name)
			bodies := map[string][]byte{journal: raw, path.Join(parent, stage): []byte("candidate"), path.Join(parent, publicationDirectory, publicationLock): nil}
			files := make(map[string]RetainedFile)
			for name, body := range bodies {
				files[name] = RetainedFile{Size: int64(len(body)), SHA256: publicationDigest(body)}
			}
			read := RetainedRecordReader(func(_ context.Context, name string, limit int64) ([]byte, error) {
				if limit != publicationJournalMaxBytes {
					t.Fatalf("unbounded journal read: %d", limit)
				}
				body, ok := bodies[name]
				if !ok {
					return nil, os.ErrNotExist
				}
				return body, nil
			})
			accepts := func(parent, destination, prefix string) bool {
				return mode != "owner-refuses" && parent == "scope" && destination == "source.json" && prefix == ".layer-"
			}
			switch mode {
			case "nil-owner":
				accepts = nil
			case "missing-read":
				read = nil
			case "missing-lock":
				delete(files, path.Join(parent, publicationDirectory, publicationLock))
			case "changed-stage":
				file := files[path.Join(parent, stage)]
				file.SHA256 = publicationDigest([]byte("different"))
				files[path.Join(parent, stage)] = file
			case "changed-journal":
				bodies[journal] = append([]byte(" "), raw...)
			}
			inactive, err := InspectRetainedPublications(t.Context(), files, read, accepts)
			if mode == "valid" {
				if err != nil || len(inactive) != 2 {
					t.Fatalf("selection: %v %v", inactive, err)
				}
			} else if err == nil {
				t.Fatal("accepted uncertain publication")
			}
		})
	}
}
