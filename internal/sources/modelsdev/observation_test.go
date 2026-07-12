package modelsdev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceAdaptersReturnCompleteObservationMetadata(t *testing.T) {
	emptyAPI := API{}
	loader := func(context.Context, string) (*API, error) {
		return &emptyAPI, nil
	}
	tests := []struct {
		name   string
		source sources.Source
	}{
		{name: "http", source: &HTTPSource{loadAPI: loader}},
		{name: "git", source: &GitSource{loadAPI: loader}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, err := test.source.Observe(context.Background())
			if err != nil {
				t.Fatalf("Observe: %v", err)
			}
			if err := observation.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if observation.SourceID != test.source.ID() {
				t.Fatalf("source = %q, want %q", observation.SourceID, test.source.ID())
			}
			if observation.Revision.Kind != sources.RevisionKindContentDigest || observation.Revision.Value == "" {
				t.Fatalf("revision = %#v", observation.Revision)
			}
			if observation.Completeness != sources.ObservationCompletenessComplete ||
				observation.Status != sources.ObservationStatusSucceeded {
				t.Fatalf("state = (%q, %q)", observation.Completeness, observation.Status)
			}
		})
	}
}

func TestInvalidIdentityQuarantineMalformedModelsDevRecordsWithCounts(t *testing.T) {
	api := API{
		"provider": Provider{
			ID: "provider", Name: "Provider",
			Models: map[string]Model{
				"bad-model": {
					ID: "", Name: "Bad Model", Description: "has promotable data but no identity",
				},
				" whitespace-id ": {
					ID: " whitespace-id ", Name: "Whitespace ID", Description: "has promotable data",
				},
				"control\n-id": {
					ID: "control\n-id", Name: "Control ID", Description: "has promotable data",
				},
				"control-name": {
					ID: "control-name", Name: "Control\nName", Description: "has promotable data",
				},
				"duplicate-key": {
					ID: "valid-model", Name: "Duplicate ID", Description: "has promotable data",
				},
				"valid-model": {
					ID: "valid-model", Name: "Valid Model", Description: "valid sibling",
				},
			},
		},
	}
	source := NewHTTPSource()
	source.loadAPI = func(context.Context, string) (*API, error) { return &api, nil }

	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate observation: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded || observation.Completeness != sources.ObservationCompletenessPartial {
		t.Fatalf("observation state = (%q, %q), want degraded partial", observation.Status, observation.Completeness)
	}
	if len(observation.Issues) != 5 {
		t.Fatalf("quarantine issues = %#v", observation.Issues)
	}
	for _, issue := range observation.Issues {
		if issue.Scope != sources.ObservationIssueScopeRecord || issue.Code != sources.ObservationIssueCodeInvalidRecord || issue.Subject == "" {
			t.Fatalf("unclassified quarantine issue: %#v", issue)
		}
	}
	provider, err := observation.Catalog.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if len(provider.Models) != 1 || provider.Models["valid-model"] == nil {
		t.Fatalf("valid sibling was not preserved: %#v", provider.Models)
	}
	if observation.Records.Accepted != 1 || observation.Records.Rejected != 5 {
		t.Fatalf("record counts = %#v, want accepted=1 rejected=5", observation.Records)
	}
}

func TestRepeatedFetchHTTPHonorsPerCallDirectory(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	writeCachedAPI(t, firstDir, cachedAPIFixture("first", "first revision"))
	writeCachedAPI(t, secondDir, cachedAPIFixture("second", "second revision"))

	first, _, err := acquireHTTPAPI(context.Background(), firstDir)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	second, _, err := acquireHTTPAPI(context.Background(), secondDir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if _, found := (*first)["provider"].Models["first"]; !found {
		t.Fatalf("first provider models = %#v", (*first)["provider"].Models)
	}
	if _, found := (*second)["provider"].Models["second"]; !found {
		t.Fatalf("second API reused process-lifetime state: %#v", (*second)["provider"].Models)
	}
}

func TestRepeatedFetchObservesChangedRevision(t *testing.T) {
	call := 0
	source := NewHTTPSource()
	source.loadAPI = func(context.Context, string) (*API, error) {
		call++
		api := API{
			"provider": Provider{ID: "provider", Models: map[string]Model{
				"model": {ID: "model", Name: "Model", Description: "revision " + strconv.Itoa(call)},
			}},
		}
		return &api, nil
	}

	first, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("first Observe: %v", err)
	}
	second, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("second Observe: %v", err)
	}
	if call != 2 {
		t.Fatalf("loader calls = %d, want 2", call)
	}
	if first.Revision == second.Revision || first.EvidenceChecksum == second.EvidenceChecksum {
		t.Fatalf("changed source produced identical revisions: first=%#v second=%#v", first.Revision, second.Revision)
	}
}

func TestRepeatedFetchGitObservesChangedRevision(t *testing.T) {
	call := 0
	source := NewGitSource()
	source.loadAPI = func(context.Context, string) (*API, error) {
		call++
		api := API{
			"provider": Provider{ID: "provider", Models: map[string]Model{
				"model": {ID: "model", Name: "Model", Description: "revision " + strconv.Itoa(call)},
			}},
		}
		return &api, nil
	}

	first, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("first Observe: %v", err)
	}
	second, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("second Observe: %v", err)
	}
	if call != 2 {
		t.Fatalf("loader calls = %d, want 2", call)
	}
	if first.Revision == second.Revision || first.EvidenceChecksum == second.EvidenceChecksum {
		t.Fatalf("changed Git source produced identical revisions: first=%#v second=%#v", first.Revision, second.Revision)
	}
}

func TestHTTPObservationClassifiesAcquisitionFallback(t *testing.T) {
	emptyAPI := API{}
	tests := []struct {
		name        string
		acquisition HTTPAcquisition
		wantScope   sources.ObservationIssueScope
		wantCode    sources.ObservationIssueCode
	}{
		{
			name:        "stale cache",
			acquisition: HTTPAcquisitionStaleCache,
			wantScope:   sources.ObservationIssueScopeStaleFallback,
			wantCode:    sources.ObservationIssueCodeStaleFallback,
		},
		{
			name:        "embedded bootstrap",
			acquisition: HTTPAcquisitionEmbeddedBootstrap,
			wantScope:   sources.ObservationIssueScopeSource,
			wantCode:    sources.ObservationIssueCodeBootstrapFallback,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := NewHTTPSource()
			source.acquireAPI = func(context.Context, string) (*API, HTTPAcquisitionResult, error) {
				return &emptyAPI, HTTPAcquisitionResult{Kind: test.acquisition}, nil
			}
			observation, err := source.Observe(context.Background())
			if err != nil {
				t.Fatalf("Observe: %v", err)
			}
			if observation.Completeness != sources.ObservationCompletenessComplete ||
				observation.Status != sources.ObservationStatusDegraded {
				t.Fatalf("state = (%q, %q)", observation.Completeness, observation.Status)
			}
			if len(observation.Issues) != 1 || observation.Issues[0].Scope != test.wantScope || observation.Issues[0].Code != test.wantCode {
				t.Fatalf("issues = %#v", observation.Issues)
			}
		})
	}
}

func TestHTTPObservationRetainsConditionalHTTPRevision(t *testing.T) {
	emptyAPI := API{}
	want := sources.Revision{Kind: sources.RevisionKindETag, Value: `"models-v1"`}
	source := NewHTTPSource()
	source.acquireAPI = func(context.Context, string) (*API, HTTPAcquisitionResult, error) {
		return &emptyAPI, HTTPAcquisitionResult{Kind: HTTPAcquisitionRevalidatedCache, Revision: want}, nil
	}

	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.Revision != want {
		t.Fatalf("observation revision = %#v, want %#v", observation.Revision, want)
	}
	if observation.Status != sources.ObservationStatusSucceeded {
		t.Fatalf("observation status = %q, want succeeded", observation.Status)
	}
}

func TestHTTPObservationRetainsEmbeddedBootstrapClassificationAcrossCacheReuse(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := &HTTPClient{
		CacheDir: filepath.Join(t.TempDir(), "models.dev"), APIURL: server.URL, Client: server.Client(),
	}
	source := NewHTTPSource()
	source.acquireAPI = func(ctx context.Context, _ string) (*API, HTTPAcquisitionResult, error) {
		result, err := client.AcquireAPI(ctx)
		if err != nil {
			return nil, HTTPAcquisitionResult{}, err
		}
		api, err := ParseAPI(client.GetAPIPath())
		return api, result, err
	}

	for call := 1; call <= 2; call++ {
		observation, err := source.Observe(context.Background())
		if err != nil {
			t.Fatalf("Observe %d: %v", call, err)
		}
		foundBootstrapFallback := false
		for _, issue := range observation.Issues {
			if issue.Scope == sources.ObservationIssueScopeSource &&
				issue.Code == sources.ObservationIssueCodeBootstrapFallback {
				foundBootstrapFallback = true
				break
			}
		}
		if observation.Status != sources.ObservationStatusDegraded || !foundBootstrapFallback {
			t.Fatalf("Observe %d lost bootstrap classification: %#v", call, observation)
		}
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want one failed request followed by classified bootstrap cache reuse", requestCount)
	}
}

func TestHTTPSourceSemanticPromotionReportsDegradedEvidence(t *testing.T) {
	api := cachedAPIFixture("valid-model", "last-known-good")
	source := NewHTTPSource()
	source.acquireAPI = func(context.Context, string) (*API, HTTPAcquisitionResult, error) {
		return &api, HTTPAcquisitionResult{
			Kind: HTTPAcquisitionStaleCache,
			Issues: []sources.ObservationIssue{{
				Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeSchemaDrift,
				Message: "upstream models.dev payload failed semantic promotion: model count regressed",
			}},
		}, nil
	}

	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded ||
		observation.Completeness != sources.ObservationCompletenessPartial {
		t.Fatalf("observation status/completeness = %q/%q", observation.Status, observation.Completeness)
	}
	if len(observation.Issues) != 2 ||
		observation.Issues[0].Code != sources.ObservationIssueCodeSchemaDrift ||
		observation.Issues[1].Code != sources.ObservationIssueCodeStaleFallback {
		t.Fatalf("semantic promotion evidence = %#v", observation.Issues)
	}
	provider, err := observation.Catalog.Provider("provider")
	if err != nil || provider.Models["valid-model"] == nil {
		t.Fatalf("last-known-good model not retained: provider=%#v err=%v", provider, err)
	}
}

func cachedAPIFixture(modelID, description string) API {
	api := API{
		"provider": Provider{ID: "provider", Name: "Provider", Models: map[string]Model{
			modelID: {ID: modelID, Name: modelID, Description: description},
		}},
	}
	for _, id := range []string{"filler-a", "filler-b", "filler-c", "filler-d"} {
		api[id] = Provider{ID: id, Name: id, Models: map[string]Model{}}
	}
	return api
}

func writeCachedAPI(t *testing.T, root string, api API) {
	t.Helper()
	data, err := json.Marshal(api)
	if err != nil {
		t.Fatalf("Marshal API: %v", err)
	}
	dir := filepath.Join(root, "models.dev")
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	apiPath := filepath.Join(dir, "api.json")
	if err := os.WriteFile(apiPath, data, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	client := NewHTTPClient(root)
	if err := client.writeCacheMetadata(apiPath, httpCacheMetadata{
		Version: httpCacheMetadataVersion, Origin: HTTPAcquisitionDownloaded,
		ContentChecksum: checksumBytes(data), ValidatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeCacheMetadata: %v", err)
	}
}
