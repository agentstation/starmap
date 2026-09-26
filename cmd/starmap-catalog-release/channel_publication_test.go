package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/publication"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

func TestPublicationChannelCommandBindsMergedInputAndCheckpoint(t *testing.T) {
	args, releasePath, repo := publicationCommandFixture(t)
	output := filepath.Join(t.TempDir(), "channel.json")
	args = append(args, "--channel-out", output)
	var report bytes.Buffer
	if err := run(args, &report); err != nil {
		t.Fatal(err)
	}
	var summary channelReport
	if err := json.Unmarshal(report.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := artifact.DecodeChannel(before)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Channel != artifact.PublicationChannelName || channel.Publication == nil || channel.Publication.SourceCommit != publicationGit(t, repo, "rev-parse", "HEAD") {
		t.Fatal("channel lost the merged source identity")
	}
	if channel.Publication.Checkpoint.Checksum != optionValue(t, args, "--channel-checkpoint-checksum") || channel.Publication.Receipt.Checksum != optionValue(t, args, "--channel-receipt-checksum") {
		t.Fatal("channel did not bind exact run inputs")
	}
	retry := filepath.Join(t.TempDir(), "retry.json")
	args = append(args, "--channel-current", output, "--previous-release-dir", releasePath, "--channel-out", retry)
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(retry)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("retry changed the accepted publication: %v", err)
	}
}

func TestPublicationChannelCommandRefusesIncompleteProof(t *testing.T) {
	args, _, repo := publicationCommandFixture(t)
	for _, name := range []string{"missing receipt provenance", "missing checkpoint provenance", "wrong receipt checksum", "wrong checkpoint checksum", "wrong source commit", "dirty embedded input", "wrong receipt artifact", "wrong mode"} {
		t.Run(name, func(t *testing.T) {
			changed := append([]string(nil), args...)
			output := filepath.Join(t.TempDir(), "refused.json")
			changed = append(changed, "--channel-out", output)
			switch name {
			case "missing receipt provenance":
				changed = append(changed, "--channel-receipt-attestation-verified=false")
			case "missing checkpoint provenance":
				changed = append(changed, "--channel-checkpoint-attestation-verified=false")
			case "wrong receipt checksum":
				changed = append(changed, "--channel-receipt-checksum", "sha256:"+strings.Repeat("0", 64))
			case "wrong checkpoint checksum":
				changed = append(changed, "--channel-checkpoint-checksum", "sha256:"+strings.Repeat("0", 64))
			case "wrong source commit":
				changed = append(changed, "--channel-source-commit", strings.Repeat("0", 40))
			case "dirty embedded input":
				path := filepath.Join(repo, "internal", "embedded", "catalog", "generation.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(raw, ' '), 0600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Error(err)
					}
				})
			case "wrong receipt artifact":
				raw, err := os.ReadFile(optionValue(t, args, "--channel-receipt"))
				if err != nil {
					t.Fatal(err)
				}
				receipt, err := artifact.DecodePublicationReceipt(raw)
				if err != nil {
					t.Fatal(err)
				}
				receipt.Artifact.PayloadChecksum = "sha256:" + strings.Repeat("0", 64)
				raw, err = artifact.EncodePublicationReceipt(receipt)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "receipt.json")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				changed = append(changed, "--channel-receipt", path, "--channel-receipt-checksum", publicationDigest(raw))
			case "wrong mode":
				changed = []string{"--inspect-dir", "unused", "--channel-receipt", "unused"}
			}
			if err := run(changed, io.Discard); err == nil {
				t.Fatal("incomplete proof advanced the channel")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("refusal wrote a channel document")
			}
		})
	}
}

func publicationCommandFixture(t *testing.T) ([]string, string, string) {
	t.Helper()
	_, releasePath, generation := promotionFixture(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "internal", "embedded"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := stagePromotionDirectory(filepath.Join(repo, "internal", "embedded", "catalog"), releasePath); err != nil {
		t.Fatal(err)
	}
	publicationGit(t, repo, "init", "--quiet")
	publicationGit(t, repo, "add", "internal/embedded/catalog")
	publicationGit(t, repo, "commit", "--quiet", "-m", "Fixture catalog")
	commit := publicationGit(t, repo, "rev-parse", "HEAD")
	state, err := publication.NewState(generation, "public-test")
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := publication.EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	checkpointPath := filepath.Join(root, "state.json")
	if err := os.WriteFile(checkpointPath, checkpoint.Data, 0600); err != nil {
		t.Fatal(err)
	}
	bundle, err := artifact.Build(generation)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	observation := generation.Manifest.SourceObservations[1]
	receipt := artifact.PublicationReceipt{SchemaVersion: artifact.PublicationReceiptSchemaVersion, RunID: "publication-test", StartedAt: observation.ObservedAt, CompletedAt: observation.ObservedAt, PolicyVersion: "public-test", FreshAcquisition: true, Artifact: artifact.PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}, Sources: []artifact.PublicationSourceReceipt{{Policy: artifact.PublicationScopePolicy{Source: observation.Source, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: "preserve"}, Attempt: "succeeded", EvidenceKind: "fresh", Observation: &observation}}}
	raw, err := artifact.EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(receiptPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"--channel-release-dir", releasePath, "--channel-published-at", generation.Manifest.GeneratedAt.Format(time.RFC3339Nano), "--channel-updated-at", receipt.CompletedAt.Add(time.Minute).Format(time.RFC3339Nano), "--channel-attestation-verified", "--channel-receipt", receiptPath, "--channel-receipt-checksum", publicationDigest(raw), "--channel-checkpoint", checkpointPath, "--channel-checkpoint-checksum", checkpoint.Checksum, "--channel-source-commit", commit, "--channel-promoted-repository", repo, "--channel-receipt-attestation-verified", "--channel-checkpoint-attestation-verified"}, releasePath, repo
}

func publicationGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	flags := []string{"-C", repo, "-c", "core.hooksPath=" + filepath.Join(repo, "fixture-hooks"), "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}
	command := exec.CommandContext(t.Context(), "git", append(flags, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func publicationDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func optionValue(t *testing.T, args []string, name string) string {
	t.Helper()
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatal("missing fixture option", name)
	return ""
}

func TestPublicationChannelRetriesHistoricalInput(t *testing.T) {
	args, releasePath, repo := publicationCommandFixture(t)
	acceptedPath := filepath.Join(t.TempDir(), "accepted.json")
	args = append(args, "--channel-out", acceptedPath)
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(acceptedPath)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := artifact.DecodeChannel(data)
	if err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(repo, "internal", "embedded", "catalog", "generation-payload.json.gz")
	publicationGit(t, repo, "rm", payload)
	publicationGit(t, repo, "commit", "--quiet", "-m", "Historical catalog fixture")
	accepted.Publication.SourceCommit = publicationGit(t, repo, "rev-parse", "HEAD")
	args = append(args, "--channel-source-commit", accepted.Publication.SourceCommit)
	if err := run(args, io.Discard); err == nil {
		t.Fatal("new publication accepted a missing compiled payload")
	}
	data, err = artifact.EncodeChannel(accepted)
	if err != nil {
		t.Fatal(err)
	}
	writePromotionFile(t, acceptedPath, data)
	retryPath := filepath.Join(t.TempDir(), "retry.json")
	args = append(args, "--channel-current", acceptedPath, "--previous-release-dir", releasePath, "--channel-out", retryPath)
	if err := run(args, io.Discard); err != nil {
		t.Fatalf("retry historical receipt: %v", err)
	}
	retried, err := os.ReadFile(retryPath)
	if err != nil || !bytes.Equal(data, retried) {
		t.Fatalf("historical retry changed channel bytes: %v", err)
	}
	for _, flag := range []string{"--channel-source-commit", "--channel-receipt-checksum", "--channel-checkpoint-checksum"} {
		t.Run(flag, func(t *testing.T) {
			changed := append(append([]string(nil), args...), flag, strings.Repeat("0", 40))
			if err := run(changed, io.Discard); err == nil {
				t.Fatal("historical retry accepted changed publication identity")
			}
		})
	}
}
