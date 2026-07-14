package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	responsefixtures "github.com/agentstation/starmap/internal/providers/fixtures/responses"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func runReplay(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("provider-fixtures replay", flag.ContinueOnError)
	providerID := flags.String("provider", "", "provider ID registered in the embedded catalog")
	sourceID := flags.String("source", "", "exact logical provider source ID")
	fixturePath := flags.String("fixture", "", "governed raw provider response fixture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *providerID == "" || *sourceID == "" || *fixturePath == "" {
		return &errors.ValidationError{Field: "arguments", Message: "provider, source, and fixture are required"}
	}
	payload, err := loadGovernedFixture(*providerID, *sourceID, *fixturePath, time.Now().UTC())
	if err != nil {
		return err
	}
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		return errors.WrapResource("load", "embedded catalog", "", err)
	}
	provider, err := builder.Provider(catalogs.ProviderID(*providerID))
	if err != nil {
		return err
	}
	models, err := decodeFixtureModels(ctx, provider, *sourceID, payload)
	if err != nil {
		return errors.WrapResource("decode", "provider fixture", *providerID, err)
	}
	if len(models) == 0 {
		return &errors.ValidationError{Field: "fixture.models", Message: "must contain at least one accepted model"}
	}
	_, _ = fmt.Fprintf(os.Stdout, "replayed provider=%s source=%s models=%d bytes=%d checksum=%s\n", *providerID, *sourceID, len(models), len(payload), responsefixtures.Checksum(payload))
	return nil
}

func loadGovernedFixture(providerID, sourceID, fixturePath string, now time.Time) ([]byte, error) {
	if responsefixtures.ProviderFromPath(fixturePath) != providerID {
		return nil, &errors.ValidationError{Field: "fixture.provider", Value: providerID, Message: "must match the governed response directory"}
	}
	if responsefixtures.SourceFromPath(fixturePath) != sourceID {
		return nil, &errors.ValidationError{Field: "fixture.source", Value: sourceID, Message: "must match the governed response directory"}
	}
	if err := responsefixtures.Verify(fixturePath, now); err != nil {
		return nil, errors.WrapResource("verify", "provider response fixture", providerID, err)
	}
	payload, err := os.ReadFile(fixturePath) //nolint:gosec // Explicit operator-controlled fixture path.
	if err != nil {
		return nil, errors.WrapIO("read", fixturePath, err)
	}
	return payload, nil
}
