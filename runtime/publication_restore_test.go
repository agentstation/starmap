package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
)

func retainedPublicationFixture(t *testing.T, parent, destination, prefix string, phase int) (map[string][]byte, map[string]RetainedFile) {
	t.Helper()
	nonce := strings.Repeat("A", 26)
	// Match the canonical field order that the private-file owner requires.
	owner := `{"entry":{"identity":"former-lock","access":"` + strings.Repeat("a", 64) + `"},"mode":384,"size":0,"modified":1,"digest":"` + hex.EncodeToString(sha256.New().Sum(nil)) + `"}`
	encodedEntry := func(identity string) string {
		return `{"identity":"` + identity + `","access":"` + strings.Repeat("a", 64) + `"}`
	}
	header := `{"header":{"version":1,"stage":"` + prefix + nonce + `","prefix":"` + prefix + `","destination":"` + destination + `","parent":` + encodedEntry("former-parent") + `,"metadata":` + encodedEntry("former-metadata") + `,"owner":` + owner + `,"journal":` + encodedEntry("former-journal") + `}}` + "\n"
	encodeRecord := func(body []byte) []byte {
		sum := sha256.Sum256(body)
		raw := `{"record":{"entry":` + encodedEntry("former-stage") + `,"mode":384,"size":` + stringMustJSON(t, len(body)) + `,"modified":1,"digest":"` + hex.EncodeToString(sum[:]) + `"}}` + "\n"
		return []byte(raw)
	}
	journal := []byte(header)
	if phase > 1 {
		journal = append(journal, encodeRecord(nil)...)
	}
	if phase > 2 {
		journal = append(journal, encodeRecord([]byte("candidate"))...)
	}
	bodies := map[string][]byte{path.Join(parent, ".record-publications", nonce+".jsonl"): journal, path.Join(parent, ".record-publications", ".owner.lock"): nil, path.Join(parent, destination): []byte("accepted")}
	if phase == 2 {
		bodies[path.Join(parent, prefix+nonce)] = nil
	}
	if phase == 3 {
		bodies[path.Join(parent, prefix+nonce)] = []byte("candidate")
	}
	return bodies, retainedPublicationFiles(bodies)
}

func stringMustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func retainedPublicationFiles(bodies map[string][]byte) map[string]RetainedFile {
	files := make(map[string]RetainedFile, len(bodies))
	for name, body := range bodies {
		sum := sha256.Sum256(body)
		files[name] = RetainedFile{Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}
	}
	return files
}
func retainedPublicationRead(bodies map[string][]byte) RetainedRecordReader {
	return func(_ context.Context, name string, _ int64) ([]byte, error) {
		body, ok := bodies[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return bytes.Clone(body), nil
	}
}

func TestRetainedPublicationsKeepOnlyJournalAndStagingInactive(t *testing.T) {
	for _, scope := range []struct{ parent, destination, prefix string }{
		{layerDirectoryName, sourceLayerFileName, ".layer-"},
		{layerDirectoryName + "/providers", "openai.json", ".layer-"},
		{layerDirectoryName + "/providers/bindings", strings.Repeat("a", 64) + ".json", ".layer-"},
		{layerDirectoryName + "/publication-inputs", strings.Repeat("b", 64) + ".json", ".input-"},
		{"github-catalog-source", strings.Repeat("c", 64) + ".json", ".state-"},
	} {
		for _, phase := range []int{1, 2, 3, 4} {
			t.Run(scope.parent+"/"+stringMustJSON(t, phase), func(t *testing.T) {
				bodies, files := retainedPublicationFixture(t, scope.parent, scope.destination, scope.prefix, phase)
				before := maps.Clone(files)
				inactive, err := InspectRetainedPublications(t.Context(), files, retainedPublicationRead(bodies))
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if phase == 2 || phase == 3 {
					want = 2
				}
				if len(inactive) != want {
					t.Fatalf("selection: %v", inactive)
				}
				for _, name := range inactive {
					if name == path.Join(scope.parent, scope.destination) || path.Base(name) == directoryLockName {
						t.Fatalf("selected accepted record: %s", name)
					}
				}
				if !reflect.DeepEqual(before, files) {
					t.Fatal("changed verified inventory")
				}
			})
		}
	}
}

func TestRetainedPublicationsRefuseUncertainEvidence(t *testing.T) {
	for _, mode := range []string{"unowned-stage", "changed-stage", "missing-lock", "nonempty-lock", "changed-journal", "unknown-metadata", "wrong-prefix", "unknown-destination", "traversal", "digest", "cancel", "nil-context", "nil-reader"} {
		t.Run(mode, func(t *testing.T) {
			bodies, files := retainedPublicationFixture(t, layerDirectoryName, sourceLayerFileName, ".layer-", 3)
			nonce := strings.Repeat("A", 26)
			stage := path.Join(layerDirectoryName, ".layer-"+nonce)
			journal := path.Join(layerDirectoryName, ".record-publications", nonce+".jsonl")
			lock := path.Join(layerDirectoryName, ".record-publications", directoryLockName)
			ctx := t.Context()
			read := retainedPublicationRead(bodies)
			switch mode {
			case "unowned-stage":
				bodies[journal] = bytes.SplitAfter(bodies[journal], []byte{'\n'})[0]
				files = retainedPublicationFiles(bodies)
			case "changed-stage":
				bodies[stage] = []byte("different")
				files = retainedPublicationFiles(bodies)
			case "missing-lock":
				delete(files, lock)
			case "nonempty-lock":
				bodies[lock] = []byte("lock")
				files = retainedPublicationFiles(bodies)
			case "changed-journal":
				bodies[journal] = append([]byte(" "), bodies[journal]...)
			case "unknown-metadata":
				bodies[path.Join(layerDirectoryName, ".record-publications", "note")] = []byte("preserve")
				files = retainedPublicationFiles(bodies)
			case "wrong-prefix":
				bodies, files = retainedPublicationFixture(t, layerDirectoryName, sourceLayerFileName, ".other-", 3)
				read = retainedPublicationRead(bodies)
			case "unknown-destination":
				bodies, files = retainedPublicationFixture(t, layerDirectoryName, "operator.json", ".layer-", 3)
				read = retainedPublicationRead(bodies)
			case "traversal":
				files["../outside"] = files[stage]
			case "digest":
				file := files[stage]
				file.SHA256 = strings.Repeat("z", 64)
				files[stage] = file
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "nil-context":
				ctx = nil
			case "nil-reader":
				read = nil
			}
			if result, err := InspectRetainedPublications(ctx, files, read); err == nil || result != nil {
				t.Fatalf("accepted uncertain evidence: %v %v", result, err)
			}
		})
	}
}
