// Package consumer is a real external module exercising offline, pinned
// catalog-artifact startup without importing acquisition, server, or remote
// implementations.
package consumer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

const pinnedArchiveSHA256 = "d4e41df425ec3cd8445232c7a8b16956c77b27821763774084c6a85b3112aede"

// ActivatePinned builds a portable fixture from the embedded generation, pins
// its exact archive digest as the offline trust root, and activates it in a
// caller-selected store without network access or provider credentials.
func ActivatePinned(ctx context.Context) error {
	embedded, err := starmap.New()
	if err != nil {
		return err
	}
	generation, err := embedded.CurrentGeneration(ctx)
	if err != nil {
		return err
	}
	bundle, err := catalogartifact.Build(generation)
	if err != nil {
		return err
	}
	release := catalogartifact.Release{
		Archive: bundle.Data,
		Checksum: []byte(
			strings.TrimPrefix(bundle.Checksum, "sha256:") +
				"  " + catalogartifact.Filename + "\n",
		),
		Attestation: bundle.Attestation,
	}
	verified, err := catalogartifact.VerifyRelease(
		ctx,
		release,
		pinnedVerifier{digest: pinnedArchiveSHA256},
	)
	if err != nil {
		return err
	}

	client, err := starmap.New(starmap.WithCatalogStore(catalogstore.NewMemory()))
	if err != nil {
		return err
	}
	publication, err := client.Activate(ctx, verified)
	if err != nil {
		return err
	}
	if !publication.Published ||
		publication.GenerationID != generation.Manifest.GenerationID ||
		client.CurrentGenerationID() != generation.Manifest.GenerationID {
		return fmt.Errorf("unexpected pinned activation: %#v", publication)
	}
	if _, err := client.Catalog().FindModel("gpt-4o"); err != nil {
		return fmt.Errorf("find pinned model: %w", err)
	}
	return nil
}

type pinnedVerifier struct {
	digest string
}

func (v pinnedVerifier) VerifyPublisher(
	ctx context.Context,
	name string,
	data []byte,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if name != catalogartifact.Filename {
		return fmt.Errorf("unexpected pinned asset %q", name)
	}
	actual := sha256.Sum256(data)
	if actualHex := fmt.Sprintf("%x", actual); actualHex != v.digest {
		return fmt.Errorf(
			"pinned archive digest mismatch: got %s, want %s",
			actualHex,
			v.digest,
		)
	}
	return nil
}
