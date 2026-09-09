package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func TestConnectedRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, false, sources.ModelsDevHTTPID, 0)
}

func TestScheduledRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, true, sources.ModelsDevHTTPID, 0)
}

func TestConfiguredSourceSelectionSkipsMetadataInApplication(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		t.Run(strconv.FormatBool(automatic), func(t *testing.T) {
			testConnectedSourceIngestion(t, automatic, sources.ModelsDevHTTPID, 0, string(sources.ProvidersID))
		})
	}
}

func TestGitConnectedRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, false, sources.ModelsDevGitID, 0)
}

func TestGitScheduledRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, true, sources.ModelsDevGitID, 0)
}

func TestGitConfiguredSourceSelectionSkipsMetadataInApplication(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		t.Run(strconv.FormatBool(automatic), func(t *testing.T) {
			testConnectedSourceIngestion(t, automatic, sources.ModelsDevGitID, 0, string(sources.ProvidersID))
		})
	}
}

func TestPeriodicRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, true, sources.ModelsDevHTTPID, 250*time.Millisecond)
}

func TestGitPeriodicRefreshIngestsProviderAndNonProviderEvidence(t *testing.T) {
	testConnectedSourceIngestion(t, true, sources.ModelsDevGitID, 250*time.Millisecond)
}

func testConnectedSourceIngestion(t *testing.T, automatic bool, metadataSource sources.ID, interval time.Duration, selection ...string) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	metadata := newApplicationMetadataFixture(t, metadataSource)
	var limit atomic.Int64
	var partial atomic.Bool
	var failed atomic.Bool
	var calls atomic.Int32
	limit.Store(131072)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failed.Load() {
			http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path != "/models" {
			t.Errorf("unexpected catalog path %s", r.URL.Path)
		}
		records := []map[string]any{{"id": "known", "object": "model", "name": "API name", "context_window": limit.Load()}}
		if partial.Load() {
			records = append(records, map[string]any{"object": "model"})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": records}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(api.Close)
	workspace := catalogs.NewEmpty()
	if err := workspace.SetAuthor(catalogs.Author{ID: "acme", Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetAuthorModel("acme", catalogs.Model{ID: "known", Name: "Known", Authors: []catalogs.Author{{ID: "acme", Name: "Acme"}}}); err != nil {
		t.Fatal(err)
	}
	credentials := testcatalog.APIKeyCredentials("ACME_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer)
	provider := catalogs.Provider{ID: "acme", Name: "Acme", Credentials: credentials, Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: api.URL + "/models", ProtocolOptions: testcatalog.OpenAIProtocolOptions(), FieldMappings: []catalogs.FieldMapping{{From: "context_window", To: "limits.context_window"}}}}, Models: map[string]*catalogs.Model{"known": {ID: "known", ModelRef: "acme/known", Name: "Reviewed offering"}}}
	if err := workspace.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "workspace")
	if err := workspace.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	payload, err := catalogs.EncodeCatalogPayload(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baselinePath, payload, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	open := func() *App {
		values := map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: baselinePath, catalogconfig.SourceStartupPolicy: "require_source", catalogconfig.AcquisitionEnabled: strconv.FormatBool(automatic), catalogconfig.AcquisitionInterval: interval.String(), catalogconfig.StartupSpread: "0s", catalogconfig.SourcePollInterval: "0s"}
		metadata.configure(values)
		if len(selection) != 0 {
			values[catalogconfig.AcquisitionSources] = selection[0]
		}
		application, err := New("test", "test", "test", "test", WithConfig(&Config{Quiet: true, CatalogPath: path, CatalogValues: values}))
		if err != nil {
			t.Fatal(err)
		}
		application.credentialResolver = sources.ProviderCredentialResolverFunc(func(_ context.Context, p *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
			return testcatalog.APIKeyMaterial(p.Credentials, "fixture-key"), nil
		})
		t.Cleanup(func() {
			if err := application.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		return application
	}
	application := open()
	connected, err := application.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if interval > 0 {
		ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
		defer cancel()
		var previousObservation string
		waitFor := func(wantLimit int64, wantTemperature bool, minimumCalls int32) {
			t.Helper()
			timer := time.NewTicker(10 * time.Millisecond)
			defer timer.Stop()
			for {
				state := connected.State()
				status := connected.Status()
				currentObservation := ""
				if status.GenerationID == state.GenerationID && status.AcquisitionHealth == runtime.HealthOK {
					for _, receipt := range status.SourceObservations {
						if receipt.Source == metadata.id && receipt.ObservationID != "" && receipt.ObservationID != previousObservation && receipt.Status == sources.ObservationStatusSucceeded && receipt.Completeness == sources.ObservationCompletenessComplete {
							metadata.assertRevision(t, receipt.Revision)
							currentObservation = receipt.ObservationID
						}
					}
				}
				provider, err := state.Catalog.Provider("acme")
				if err == nil {
					model := provider.Models["known"]
					if currentObservation != "" && model != nil && model.Limits != nil && model.Limits.ContextWindow == wantLimit && model.Features != nil && model.Features.Temperature == wantTemperature && model.Description == "Metadata fixture" && calls.Load() >= minimumCalls && metadata.count(t) >= minimumCalls {
						generation, err := connected.Client().Generation(ctx, state.GenerationID)
						if err != nil {
							t.Fatal(err)
						}
						for _, field := range []struct {
							name   string
							source sources.ID
						}{{"limits.context_window", sources.ProvidersID}, {"Features.temperature", metadata.id}} {
							evidence := state.Catalog.Provenance().FindModelField("acme", "known", field.name)
							if len(evidence) != 1 || evidence[0].Source != field.source {
								t.Fatalf("periodic %s lost provenance", field.name)
							}
							bound := false
							for _, link := range generation.Manifest.SourceObservations {
								if link.Source == field.source && link.ObservationID == evidence[0].ObservationID && link.EvidenceChecksum == evidence[0].EvidenceChecksum {
									if field.source == metadata.id {
										metadata.assertRevision(t, link.Revision)
									}
									bound = true
								}
							}
							if !bound {
								t.Fatalf("periodic %s lost immutable receipt", field.name)
							}
						}
						previousObservation = currentObservation
						return
					}
				}
				select {
				case <-ctx.Done():
					t.Fatalf("periodic acquisition did not publish limit %d with %d source calls: %v", wantLimit, minimumCalls, ctx.Err())
				case <-timer.C:
				}
			}
		}
		waitFor(131072, false, 1)
		limit.Store(262144)
		wantTemperature := false
		if metadata.git == nil {
			metadata.temperature.Store(true)
			wantTemperature = true
		}
		directories, err := application.SourceDirectories()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(directories.Cache, "models.dev")); err != nil {
			t.Fatal(err)
		}
		waitFor(262144, wantTemperature, 2)
		return
	}
	if automatic {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		timer := time.NewTicker(10 * time.Millisecond)
		defer timer.Stop()
		for connected.Status().AcquisitionHealth == runtime.HealthUnknown {
			select {
			case <-ctx.Done():
				t.Fatal("automatic source acquisition did not finish")
			case <-timer.C:
			}
		}
	} else {
		report, err := connected.Sync(t.Context(), "acme")
		if err != nil {
			t.Fatal(err)
		}
		if report.Succeeded != 1 {
			t.Fatalf("provider acquisition report=%+v", report)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", calls.Load())
	}
	wantMetadataCalls, wantDescription := int32(1), "Metadata fixture"
	if len(selection) != 0 {
		wantMetadataCalls, wantDescription = 0, ""
	}
	if metadata.count(t) != wantMetadataCalls {
		t.Fatalf("metadata calls=%d, want %d", metadata.count(t), wantMetadataCalls)
	}
	observed, err := connected.State().Catalog.Provider("acme")
	if err != nil {
		t.Fatal(err)
	}
	model := observed.Models["known"]
	if model == nil || model.Description != wantDescription || model.Limits == nil || model.Limits.ContextWindow != 131072 {
		t.Fatalf("connected catalog did not combine provider and metadata facts: %+v", model)
	}
	assertCurrent := func(wantTemperature bool) {
		t.Helper()
		provider, err := connected.State().Catalog.Provider("acme")
		if err != nil {
			t.Fatal(err)
		}
		current := provider.Models["known"]
		if current == nil || current.Limits == nil || current.Limits.ContextWindow != 262144 || current.Description != wantDescription {
			t.Fatalf("connected refresh lost the selected facts: %+v", current)
		}
		if len(selection) == 0 && (current.Features == nil || current.Features.Temperature != wantTemperature) {
			t.Fatalf("metadata temperature = %+v, want %t", current.Features, wantTemperature)
		}
		if len(selection) == 0 {
			evidence := connected.State().Catalog.Provenance().FindModelField("acme", "known", "Features.temperature")
			if len(evidence) != 1 || evidence[0].Source != metadata.id {
				t.Fatal("changed metadata lost its source provenance")
			}
			generation, err := connected.Client().Generation(t.Context(), connected.State().GenerationID)
			if err != nil {
				t.Fatal(err)
			}
			bound := false
			for _, link := range generation.Manifest.SourceObservations {
				if link.Source == metadata.id && link.ObservationID == evidence[0].ObservationID && link.EvidenceChecksum == evidence[0].EvidenceChecksum {
					metadata.assertRevision(t, link.Revision)
					bound = true
				}
			}
			if !bound {
				t.Fatal("changed metadata has no matching immutable receipt")
			}
		}
	}
	clearMetadata := func() {
		t.Helper()
		directories, err := application.SourceDirectories()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(directories.Cache, "models.dev")); err != nil {
			t.Fatal(err)
		}
	}
	restartForPin := func() {
		t.Helper()
		if metadata.git == nil {
			return
		}
		if err := application.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		automatic = false
		application = open()
		var err error
		connected, err = application.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
	}
	initial := connected.State()
	limit.Store(262144)
	metadata.temperature.Store(true)
	partial.Store(true)
	clearMetadata()
	restartForPin()
	report, err := connected.Sync(t.Context(), "acme")
	if err != nil || report.Health != runtime.HealthDegraded {
		t.Fatalf("partial connected refresh: report=%+v error=%v", report, err)
	}
	assertCurrent(true)
	changed := connected.State()
	if changed.GenerationID == initial.GenerationID || changed.PayloadChecksum == initial.PayloadChecksum {
		t.Fatal("changed provider and metadata inputs kept the old generation")
	}
	generation, err := connected.Client().Generation(t.Context(), changed.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	providerReceipts := changed.Catalog.Provenance().FindModelField("acme", "known", "limits.context_window")
	if len(providerReceipts) != 1 || !generation.Manifest.Degraded {
		t.Fatal("partial provider facts lost their receipt or degraded status")
	}
	bound := false
	for _, link := range generation.Manifest.SourceObservations {
		if link.Source == sources.ProvidersID && link.ObservationID == providerReceipts[0].ObservationID && link.EvidenceChecksum == providerReceipts[0].EvidenceChecksum && link.Status == sources.ObservationStatusDegraded && link.Completeness == sources.ObservationCompletenessPartial {
			bound = true
		}
	}
	if !bound {
		t.Fatal("partial provider facts have no matching immutable manifest receipt")
	}
	failed.Store(true)
	metadata.temperature.Store(false)
	clearMetadata()
	restartForPin()
	report, err = connected.Sync(t.Context(), "acme")
	if report.Failed != 1 || report.Health == runtime.HealthOK {
		t.Fatalf("failed provider reported success: report=%+v error=%v", report, err)
	}
	assertCurrent(false)
	if len(selection) != 0 && metadata.count(t) != 0 {
		t.Fatal("disabled metadata source ran during a later refresh")
	}
	accepted := connected.State()
	providerCallsBefore, metadataCallsBefore := calls.Load(), metadata.count(t)
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	automatic = false
	application = open()
	connected, err = application.Runtime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	assertCurrent(false)
	if connected.State().GenerationID != accepted.GenerationID || connected.State().PayloadChecksum != accepted.PayloadChecksum {
		t.Fatal("restart changed the retained catalog generation")
	}
	if calls.Load() != providerCallsBefore || metadata.count(t) != metadataCallsBefore {
		t.Fatal("restart contacted a disabled acquisition source")
	}
}
