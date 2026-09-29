package privatefiles

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func retainedJournalFixture(t *testing.T) (string, []byte) {
	t.Helper()
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	writer, err := directory.acquirePublicationWriter(t.Context(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.close()
	journal, err := writer.newJournal("source.json", ".layer-")
	if err != nil {
		t.Fatal(err)
	}
	defer journal.file.Close()
	stage, err := CreateFile(writer.root, journal.header.Stage)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	empty, err := publicationRecordOf(writer.root, journal.header.Stage, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.append(writer, publicationEvent{Record: &empty}); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Write([]byte("candidate")); err != nil {
		t.Fatal(err)
	}
	if err := stage.Sync(); err != nil {
		t.Fatal(err)
	}
	written, err := publicationRecordOf(writer.root, journal.header.Stage, 128)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.append(writer, publicationEvent{Record: &written}); err != nil {
		t.Fatal(err)
	}
	return journal.name, bytes.Clone(journal.contents)
}

func TestRetainedPublicationUsesRealJournalWithoutNativeCleanup(t *testing.T) {
	name, raw := retainedJournalFixture(t)
	lines := bytes.SplitAfter(raw, []byte{'\n'})
	for _, count := range []int{1, 2, 3} {
		part := bytes.Join(lines[:count], nil)
		got, err := InspectRetainedPublication(name, part)
		if err != nil {
			t.Fatal(err)
		}
		if got.Destination != "source.json" || got.Prefix != ".layer-" || got.Stage != ".layer-"+strings.TrimSuffix(name, ".jsonl") || got.HasRecord != (count > 1) {
			t.Fatalf("unexpected selection: %+v", got)
		}
		if count == 3 && (got.Size != 9 || got.SHA256 != publicationDigest([]byte("candidate"))) {
			t.Fatalf("lost bytes: %+v", got)
		}
	}
}

func TestRetainedPublicationRefusesMalformedEvidence(t *testing.T) {
	name, raw := retainedJournalFixture(t)
	for _, mode := range []string{"name", "empty", "oversized", "torn", "whitespace", "extra", "unknown", "duplicate", "version", "stage", "parent", "access", "owner", "state-identity", "state-digest", "first-state"} {
		t.Run(mode, func(t *testing.T) {
			candidate, journalName := bytes.Clone(raw), name
			var events []publicationEvent
			for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'}) {
				var event publicationEvent
				if err := json.Unmarshal(line, &event); err != nil {
					t.Fatal(err)
				}
				events = append(events, event)
			}
			switch mode {
			case "name":
				journalName = "../" + name
			case "empty":
				candidate = nil
			case "oversized":
				candidate = bytes.Repeat([]byte{'x'}, publicationJournalMaxBytes+1)
			case "torn":
				candidate = candidate[:len(candidate)-1]
			case "whitespace":
				candidate = append([]byte{' '}, candidate...)
			case "extra":
				candidate = append(candidate, []byte("{}\n")...)
			case "unknown":
				candidate = bytes.Replace(candidate, []byte(`{"header":`), []byte(`{"unknown":1,"header":`), 1)
			case "duplicate":
				candidate = bytes.Replace(candidate, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
			default:
				switch mode {
				case "version":
					events[0].Header.Version++
				case "stage":
					events[0].Header.Stage = "source.json"
				case "parent":
					events[0].Header.Parent.Identity = ""
				case "access":
					events[0].Header.Metadata.Access = strings.Repeat("z", 64)
				case "owner":
					events[0].Header.Owner.Size = 1
				case "state-identity":
					events[2].Record.Entry.Identity = "different"
				case "state-digest":
					events[2].Record.Digest = strings.Repeat("z", 64)
				case "first-state":
					events[1].Record.Size = 1
				}
				candidate = nil
				for _, event := range events {
					line, err := json.Marshal(event)
					if err != nil {
						t.Fatal(err)
					}
					candidate = append(candidate, line...)
					candidate = append(candidate, '\n')
				}
			}
			if _, err := InspectRetainedPublication(journalName, candidate); err == nil {
				t.Fatal("accepted malformed retained evidence")
			}
		})
	}
}
