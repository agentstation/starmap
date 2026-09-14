package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/catalog/publication"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

const (
	promotionGitTimeout            = time.Minute
	publicationVerificationTimeout = time.Hour
)

type channelPublicationOptions struct {
	receiptPath, receiptChecksum        string
	checkpointPath, checkpointChecksum  string
	sourceCommit, repository            string
	receiptAttested, checkpointAttested bool
}

func (o channelPublicationOptions) requested() bool {
	return o.receiptPath != "" || o.receiptChecksum != "" || o.checkpointPath != "" || o.checkpointChecksum != "" || o.sourceCommit != "" || o.repository != "" || o.receiptAttested || o.checkpointAttested
}

func verifyChannelPublication(o channelPublicationOptions, release releaseReport) (artifact.PublicationPromotion, error) {
	if o.receiptPath == "" || o.receiptChecksum == "" || o.checkpointPath == "" || o.checkpointChecksum == "" || o.repository == "" || o.sourceCommit == "" || !o.receiptAttested || !o.checkpointAttested {
		return artifact.PublicationPromotion{}, channelFlagError("publication", "requires verified receipt, checkpoint, and merged source inputs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), publicationVerificationTimeout)
	defer cancel()
	if err := verifyPromotionCommit(ctx, o.repository, o.sourceCommit); err != nil {
		return artifact.PublicationPromotion{}, err
	}
	if _, err := verifyPromotionDirectory(filepath.Join(o.repository, "internal", "embedded", "catalog"), release.Directory); err != nil {
		return artifact.PublicationPromotion{}, err
	}
	receipt, err := readPublicationInput(o.receiptPath, artifact.MaxPublicationReceiptBytes)
	if err != nil {
		return artifact.PublicationPromotion{}, err
	}
	checkpoint, err := readPublicationInput(o.checkpointPath, artifact.MaxPublicationCheckpointBytes)
	if err != nil {
		return artifact.PublicationPromotion{}, err
	}
	state, err := publication.RestoreState(ctx, checkpoint, o.checkpointChecksum)
	if err != nil {
		return artifact.PublicationPromotion{}, err
	}
	stateBundle, err := artifact.Build(state.Generation())
	if err != nil {
		return artifact.PublicationPromotion{}, err
	}
	if stateBundle.Checksum != release.ArchiveChecksum {
		return artifact.PublicationPromotion{}, channelFlagError("checkpoint", "does not retain the selected catalog artifact")
	}
	if err := verifyPromotionCommit(ctx, o.repository, o.sourceCommit); err != nil {
		return artifact.PublicationPromotion{}, err
	}
	return artifact.PublicationPromotion{Receipt: receipt, ReceiptChecksum: o.receiptChecksum,
		Artifact: artifact.PublicationArtifact{GenerationID: release.GenerationID, CatalogChecksum: release.SemanticChecksum,
			PayloadChecksum: release.PayloadChecksum, ArchiveChecksum: release.ArchiveChecksum},
		SourceCommit: o.sourceCommit, ReceiptAttestationVerified: true, EmbeddingVerified: true, CheckpointVerified: true,
		Checkpoint: artifact.ChannelAsset{Name: artifact.PublicationCheckpointFilename, MediaType: artifact.PublicationCheckpointMediaType,
			Checksum: o.checkpointChecksum, SizeBytes: int64(len(checkpoint))}}, nil
}

func verifyPromotionCommit(ctx context.Context, repository, expected string) error {
	ctx, cancel := context.WithTimeout(ctx, promotionGitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-C", repository, "rev-parse", "--verify", "HEAD")
	actual, err := command.Output()
	if err != nil || strings.TrimSpace(string(actual)) != expected {
		return channelFlagError("source_commit", "checkout does not match the selected source commit")
	}
	command = exec.CommandContext(ctx, "git", "-C", repository, "status", "--porcelain", "--untracked-files=all", "--ignored=matching", "--", "internal/embedded/catalog")
	dirty, err := command.Output()
	if err != nil || len(dirty) != 0 {
		return channelFlagError("promoted_repository", "embedded input must match the clean source commit")
	}
	return nil
}

func readPublicationInput(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, channelFlagError("publication_input", "must name a bounded regular file")
	}
	file, err := os.Open(path) //nolint:gosec // Explicit publisher input with a strict byte bound.
	if err != nil {
		return nil, pkgerrors.WrapIO("open publication input", path, err)
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, channelFlagError("publication_input", "must be a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, pkgerrors.WrapIO("read publication input", path, err)
	}
	if int64(len(data)) > limit {
		return nil, channelFlagError("publication_input", "exceeds the input byte limit")
	}
	return data, nil
}
