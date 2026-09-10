package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChannelRequiresPublishedPredecessor(t *testing.T) {
	store, _ := releaseFixtureStore(t)
	var staged bytes.Buffer
	if err := run([]string{"--generation-store", store, "--output-dir", t.TempDir()}, &staged); err != nil {
		t.Fatal(err)
	}
	var release releaseReport
	if err := json.Unmarshal(staged.Bytes(), &release); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(t.TempDir(), "channel.json")
	args := []string{"--channel-release-dir", release.Directory, "--channel-published-at", "2026-09-10T00:00:00Z", "--channel-updated-at", "2026-09-10T01:00:00Z", "--channel-out", current, "--channel-attestation-verified"}
	if err := run(args, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(t.TempDir(), "next.json")
	args = []string{"--channel-release-dir", release.Directory, "--channel-published-at", "2026-09-10T00:00:00Z", "--channel-updated-at", "2026-09-10T02:00:00Z", "--channel-current", current, "--channel-out", next, "--channel-attestation-verified"}
	if err := run(args, &bytes.Buffer{}); err == nil {
		t.Fatal("advanced an existing channel without its selected release history")
	}
	if _, err := os.Stat(next); !os.IsNotExist(err) {
		t.Fatal("unverified predecessor wrote a channel document")
	}
}

func TestPublishedCanonicalAliasSuccessor(t *testing.T) {
	for _, change := range []string{"unchanged", "removed", "dropped", "reassigned", "wrong_predecessor", "missing_channel", "empty_channel", "asset_mismatch"} {
		t.Run(change, func(t *testing.T) {
			_, previous := aliasReleaseFixture(t, "unchanged")
			current := filepath.Join(t.TempDir(), "channel.json")
			if err := run([]string{"--channel-release-dir", previous.Directory, "--channel-published-at", "2026-09-10T00:00:00Z", "--channel-updated-at", "2026-09-10T01:00:00Z", "--channel-out", current, "--channel-attestation-verified"}, io.Discard); err != nil {
				t.Fatal(err)
			}
			candidateState := change
			if change == "wrong_predecessor" || change == "missing_channel" || change == "empty_channel" || change == "asset_mismatch" {
				candidateState = "removed"
			}
			store, candidate := aliasReleaseFixture(t, candidateState)
			if change == "wrong_predecessor" {
				previous = candidate
			}
			if change == "missing_channel" {
				current = filepath.Join(t.TempDir(), "missing.json")
			}
			if change == "empty_channel" {
				if err := os.WriteFile(current, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if change == "asset_mismatch" {
				data, err := os.ReadFile(current)
				if err != nil {
					t.Fatal(err)
				}
				var document map[string]any
				if err := json.Unmarshal(data, &document); err != nil {
					t.Fatal(err)
				}
				document["assets"].([]any)[0].(map[string]any)["size_bytes"] = float64(1)
				data, err = json.Marshal(document)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(current, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, mode := range []string{"release", "channel"} {
				t.Run(mode, func(t *testing.T) {
					output := filepath.Join(t.TempDir(), "candidate")
					args := []string{"--channel-current", current, "--previous-release-dir", previous.Directory}
					if mode == "release" {
						args = append(args, "--generation-store", store, "--output-dir", output)
					} else {
						args = append(args, "--channel-release-dir", candidate.Directory, "--channel-published-at", "2026-09-10T00:00:00Z", "--channel-updated-at", "2026-09-10T02:00:00Z", "--channel-out", output, "--channel-attestation-verified")
					}
					err := run(args, io.Discard)
					if change == "unchanged" || change == "removed" {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					if err == nil {
						t.Fatal("publisher accepted invalid predecessor or rename history")
					}
					if _, err := os.Stat(output); !os.IsNotExist(err) {
						t.Fatal("rejected predecessor wrote publication output")
					}
				})
			}
		})
	}
}

func aliasReleaseFixture(t *testing.T, state string) (string, releaseReport) {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "author", Name: "Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"current", "other"} {
		if err := builder.SetAuthorModel(author.ID, catalogs.Model{ID: id, Name: id, Authors: []catalogs.Author{author}}); err != nil {
			t.Fatal(err)
		}
	}
	aliases := []catalogs.CanonicalAlias{{ID: "author/old", TargetID: "author/current", PublisherID: "baseline", State: catalogs.CanonicalAliasActive}}
	switch state {
	case "removed":
		aliases[0].State = catalogs.CanonicalAliasRemoved
	case "reassigned":
		aliases[0].TargetID = "author/other"
	case "dropped":
		aliases = nil
	}
	if err := builder.SetCanonicalAliasRecords(aliases); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	_, generation := releaseFixtureStore(t)
	generation.Payload, err = catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
	generation.Manifest.GenerationID = "alias-release-" + state
	generation.Manifest.GeneratedAt = time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "store")
	store, err := storage.NewFilesystem(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatal(err)
	}
	var result bytes.Buffer
	if err := run([]string{"--generation-store", path, "--output-dir", t.TempDir()}, &result); err != nil {
		t.Fatal(err)
	}
	var report releaseReport
	if err := json.Unmarshal(result.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return path, report
}
