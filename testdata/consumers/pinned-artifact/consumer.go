// Package consumer is a real external module exercising offline, pinned
// catalog-artifact startup without importing acquisition, server, or remote
// implementations.
package consumer

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const pinnedArchiveSHA256 = "b21b6b44606fcc0a3932bfe2e75ffbea47dbd558df9e84abb261e131681f4988"

//go:embed testdata/starmap-catalog.*
var fixtureFiles embed.FS

// ActivatePinned verifies a fixed portable fixture against its offline trust root.
// It activates the fixture in a caller-selected store without network access
// or provider credentials. The fixture is independent of the current embedded catalog.
func ActivatePinned(ctx context.Context) error {
	release, err := pinnedRelease()
	if err != nil {
		return err
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
	if !publication.Published ||
		publication.GenerationID != verified.Manifest.GenerationID ||
		state.GenerationID != verified.Manifest.GenerationID ||
		state.PayloadChecksum != verified.Manifest.Payload.Checksum ||
		state.Catalog == initial.Catalog ||
		durable.Manifest.GenerationID != verified.Manifest.GenerationID {
		return fmt.Errorf("unexpected pinned activation: %#v", publication)
	}
	if _, err := client.Catalog().FindModel("fixture/pinned"); err != nil {
		return fmt.Errorf("find pinned model: %w", err)
	}
	retry, err := client.Activate(ctx, verified)
	if err != nil {
		return err
	}
	if retry.Published || client.CurrentCatalogState().Catalog != state.Catalog {
		return fmt.Errorf("duplicate pinned activation changed the catalog")
	}
	restarted, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		return err
	}
	if restarted.CurrentCatalogState().GenerationID != verified.Manifest.GenerationID {
		return fmt.Errorf("pinned generation did not survive restart")
	}
	if _, err := restarted.Catalog().FindModel("fixture/pinned"); err != nil {
		return err
	}
	return nil
}

func pinnedRelease() (artifact.Release, error) {
	archive, err := fixtureFiles.ReadFile("testdata/" + artifact.Filename)
	if err != nil {
		return artifact.Release{}, err
	}
	checksum, err := fixtureFiles.ReadFile("testdata/" + artifact.Filename + ".sha256")
	if err != nil {
		return artifact.Release{}, err
	}
	statement, err := fixtureFiles.ReadFile("testdata/" + artifact.AttestationFilename)
	if err != nil {
		return artifact.Release{}, err
	}
	return artifact.Release{Archive: archive, Checksum: checksum, Attestation: statement}, nil
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
