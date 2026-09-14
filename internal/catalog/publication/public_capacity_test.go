package publication

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestPublicPublicationProfileRetainsBoundedState(t *testing.T) {
	if testing.Short() {
		t.Skip("full public profile capacity requires the publication qualification gate")
	}
	profile := publicPublicationProfile(t)
	baseline, err := bootstrap.Generation()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeCatalogGeneration(baseline)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../embedded/sources/models.dev/api.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadataRequests, providerRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		metadataRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(raw); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	sourceDirectory := t.TempDir()
	client := modelsdev.NewHTTPClient(sourceDirectory)
	client.APIURL, client.Client = server.URL, server.Client()
	if _, err := client.AcquireAPI(t.Context()); err != nil {
		t.Fatal(err)
	}
	factory := func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		models := make([]catalogs.Model, 0, len(provider.Models))
		for _, model := range provider.Models {
			models = append(models, catalogs.DeepCopyModel(*model))
		}
		return publicCapacityClient{models: models, calls: &providerRequests}, nil
	}
	resolver := sources.ProviderCredentialResolverFunc(func(_ context.Context, provider *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		selected := provider.Credentials.CatalogAcquisition.Alternatives
		if len(selected) != 1 {
			return sources.ProviderCredentialMaterial{}, &pkgerrors.ConfigError{Message: "fixture requires one selected credential profile"}
		}
		for _, declared := range provider.Credentials.Profiles {
			if declared.ID == selected[0] {
				var values map[catalogs.ProviderCredentialFieldID]string
				if declared.ID != "public" {
					values = map[catalogs.ProviderCredentialFieldID]string{"api-key": "public-capacity-fixture"}
				}
				return sources.NewProviderCredentialMaterial(declared, values, sources.ProviderCredentialMetadata{}), nil
			}
		}
		return sources.ProviderCredentialMaterial{}, &pkgerrors.ConfigError{Message: "fixture credential profile is absent"}
	})
	producer, err := NewProducer(profile, factory, resolver)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := producer.Collect(t.Context(), catalog, nil, pkgsync.WithCatalogPath(t.TempDir()), pkgsync.WithSourcesDir(sourceDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if !collection.Decision.Allowed || len(collection.Decision.Inputs) != len(profile.Scopes) || providerRequests.Load() != 12 || metadataRequests.Load() != 1 {
		for _, attempt := range collection.Run.Attempts {
			t.Logf("scope=%+v outcome=%s", attempt.Scope.key(), attempt.Outcome)
			if attempt.Observation != nil {
				t.Logf("completeness=%s status=%s records=%+v issues=%+v", attempt.Observation.Completeness, attempt.Observation.Status, attempt.Observation.Records, attempt.Observation.Issues)
			}
		}
		t.Fatalf("complete public acquisition: allowed=%t inputs=%d provider_calls=%d metadata_calls=%d", collection.Decision.Allowed, len(collection.Decision.Inputs), providerRequests.Load(), metadataRequests.Load())
	}
	state, err := NewState(baseline, "public-capacity")
	if err != nil {
		t.Fatal(err)
	}
	start := collection.Run.CompletedAt
	for step := range 8 {
		at := start.Add(time.Duration(step) * 4 * time.Hour)
		run := Run{StartedAt: at, CompletedAt: at}
		for _, input := range collection.Decision.Inputs {
			scope := Scope{Source: input.SourceID, Binding: input.ProviderBinding}
			attempt := Attempt{Scope: scope, Outcome: Failed}
			if step != 4 {
				observation, err := sources.NewObservation(input.SourceID, input.Catalog, sources.ObservationMetadata{
					ObservedAt: at, ProviderBinding: input.ProviderBinding, Revision: input.Revision,
					Completeness: input.Completeness, Status: input.Status, Records: input.Records, Issues: input.Issues,
				})
				if err != nil {
					t.Fatal(err)
				}
				attempt.Outcome, attempt.Observation = Succeeded, &observation
			}
			run.Attempts = append(run.Attempts, attempt)
		}
		run.Retained, err = retainedForProfile(profile, state.history)
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := preparePublication(t.Context(), state, profile, run, fmt.Sprintf("public-capacity-%d", step))
		if err != nil {
			t.Fatalf("run %d: %v", step, err)
		}
		checkpoint, err := EncodeState(prepared.Next)
		if err != nil {
			t.Fatal(err)
		}
		state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
		if err != nil {
			t.Fatalf("restore run %d: %v", step, err)
		}
		receipt, err := artifact.DecodePublicationReceipt(prepared.Receipt.Data)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.FreshAcquisition != (step != 4) || len(receipt.Sources) != len(profile.Scopes) {
			t.Fatal("public receipt changed acquisition freshness or source coverage")
		}
		if step == 4 && !prepared.ReusedArtifact {
			t.Fatal("source outage replaced the accepted public artifact")
		}
		if len(state.history) > 2*len(profile.Scopes) {
			t.Fatalf("stable public sources retained %d inputs across %d scopes", len(state.history), len(profile.Scopes))
		}
		after, err := catalogs.DecodeCatalogGeneration(state.Generation())
		if err != nil {
			t.Fatal(err)
		}
		if len(after.MembershipScopes()) != 12 {
			t.Fatal("checkpoint lost a configured public provider scope")
		}
		t.Logf("run=%d scopes=%d inputs=%d checkpoint_bytes=%d catalog_bytes=%d reused=%t", step+1, len(profile.Scopes), len(state.history), len(checkpoint.Data), len(state.Generation().Payload), prepared.ReusedArtifact)
	}
}

type publicCapacityClient struct {
	models []catalogs.Model
	calls  *atomic.Int32
}

func (c publicCapacityClient) ListModels(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	c.calls.Add(1)
	return c.models, nil
}
