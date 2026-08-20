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
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const pinnedArchiveSHA256 = "728c050e7c0fd232a7d8f6d41dc40d4c20d9ceb28ac3522c037056235c344731"

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
	bundle, err := artifact.Build(generation)
	if err != nil {
		return err
	}
	release := artifact.Release{
		Archive: bundle.Data,
		Checksum: []byte(
			strings.TrimPrefix(bundle.Checksum, "sha256:") +
				"  " + artifact.Filename + "\n",
		),
		Attestation: bundle.Attestation,
	}
	verified, err := artifact.VerifyRelease(
		ctx,
		release,
		pinnedVerifier{digest: pinnedArchiveSHA256},
	)
	if err != nil {
		return err
	}

	store := storage.NewMemory()
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		return err
	}
	initial := client.CurrentCatalogState()
	publication, err := client.Activate(ctx, verified)
	if err != nil {
		return err
	}
	state := client.CurrentCatalogState()
	durable, err := store.Current(ctx)
	if err != nil {
		return err
	}
	if publication.Published ||
		publication.GenerationID != generation.Manifest.GenerationID ||
		state.GenerationID != generation.Manifest.GenerationID ||
		state.PayloadChecksum != generation.Manifest.Payload.Checksum ||
		state.Catalog != initial.Catalog ||
		durable.Manifest.GenerationID != generation.Manifest.GenerationID {
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
	if name != artifact.Filename {
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
