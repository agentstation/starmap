package artifact

import (
	"bytes"
	"context"
	"reflect"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Release contains the three immutable assets published for one catalog
// generation. Publisher provenance is channel-specific and is therefore
// verified through PublisherVerifier rather than encoded as an unsigned field.
type Release struct {
	Archive     []byte
	Checksum    []byte
	Attestation []byte
}

// PublisherVerifier authenticates exact archive bytes to the caller's expected
// publisher. A GitHub Release implementation, for example, should require the
// expected repository and signer workflow when verifying build provenance.
//
// VerifyPublisher must return nil only when data is authenticated as the exact
// contents of name. Implementations own credentials, clients, trust policy,
// network access, and lifecycle.
type PublisherVerifier interface {
	VerifyPublisher(ctx context.Context, name string, data []byte) error
}

// VerifyRelease checks the detached checksum, archive statement, generation
// compatibility, and channel-specific publisher identity before returning the
// exact immutable generation. It performs no activation or persistence.
func VerifyRelease(
	ctx context.Context,
	release Release,
	verifier PublisherVerifier,
) (catalogs.Generation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	if verifier == nil || isNilPublisherVerifier(verifier) {
		return catalogs.Generation{}, &errors.ValidationError{
			Field:   "catalog_artifact.publisher_verifier",
			Message: "is required",
		}
	}

	wantChecksum := strings.TrimPrefix(checksum(release.Archive), "sha256:") +
		"  " + Filename + "\n"
	if !bytes.Equal(release.Checksum, []byte(wantChecksum)) {
		return catalogs.Generation{}, artifactValidation(
			"release.checksum",
			strings.TrimSpace(string(release.Checksum)),
			"does not match archive bytes",
		)
	}
	generation, err := Open(release.Archive, release.Attestation)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion ||
		!generation.Manifest.ConsumerCompatibility.SupportsSchema(
			catalogs.CurrentCatalogSchemaVersion,
		) {
		return catalogs.Generation{}, artifactValidation(
			"release.compatibility",
			generation.Manifest.SchemaVersion,
			"is not compatible with this Starmap catalog schema",
		)
	}
	if err := verifier.VerifyPublisher(
		ctx,
		Filename,
		append([]byte(nil), release.Archive...),
	); err != nil {
		return catalogs.Generation{}, errors.WrapResource(
			"verify",
			"catalog artifact publisher",
			generation.Manifest.GenerationID,
			err,
		)
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	return generation, nil
}

func isNilPublisherVerifier(verifier PublisherVerifier) bool {
	value := reflect.ValueOf(verifier)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
