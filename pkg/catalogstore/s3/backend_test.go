package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

const testBucket = "catalogs"

type staticCredentials struct{}

func (staticCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Source:          "test",
	}, nil
}

type protocolObject struct {
	data []byte
	etag string
}

type protocolRequest struct {
	method      string
	key         string
	ifMatch     string
	ifNoneMatch string
}

type protocolServer struct {
	mu              sync.Mutex
	objects         map[string]protocolObject
	requests        []protocolRequest
	fail            func(string, string) int
	rejectCondition bool
}

func newProtocolServer() *protocolServer {
	return &protocolServer{objects: make(map[string]protocolObject)}
}

func (s *protocolServer) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	key, ok := protocolKey(request.URL.Path)
	if !ok {
		writeS3Error(response, http.StatusNotFound, "NoSuchBucket")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, protocolRequest{
		method:      request.Method,
		key:         key,
		ifMatch:     request.Header.Get("If-Match"),
		ifNoneMatch: request.Header.Get("If-None-Match"),
	})
	if s.fail != nil {
		if status := s.fail(request.Method, key); status != 0 {
			writeS3Error(response, status, http.StatusText(status))
			return
		}
	}
	switch request.Method {
	case http.MethodGet:
		s.get(response, key)
	case http.MethodPut:
		s.put(response, request, key)
	default:
		writeS3Error(response, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func (s *protocolServer) get(response http.ResponseWriter, key string) {
	object, found := s.objects[key]
	if !found {
		writeS3Error(response, http.StatusNotFound, "NoSuchKey")
		return
	}
	response.Header().Set("ETag", object.etag)
	response.Header().Set("Content-Length", fmt.Sprint(len(object.data)))
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(object.data)
}

func (s *protocolServer) put(response http.ResponseWriter, request *http.Request, key string) {
	if s.rejectCondition && (request.Header.Get("If-Match") != "" ||
		request.Header.Get("If-None-Match") != "") {
		writeS3Error(response, http.StatusNotImplemented, "NotImplemented")
		return
	}
	existing, found := s.objects[key]
	switch {
	case request.Header.Get("If-None-Match") == "*" && found:
		writeS3Error(response, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	case request.Header.Get("If-Match") != "" &&
		(!found || request.Header.Get("If-Match") != existing.etag):
		writeS3Error(response, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	case request.Header.Get("If-None-Match") == "" && request.Header.Get("If-Match") == "":
		writeS3Error(response, http.StatusBadRequest, "ConditionalWriteRequired")
		return
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		writeS3Error(response, http.StatusBadRequest, "InvalidBody")
		return
	}
	digest := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	s.objects[key] = protocolObject{data: append([]byte(nil), data...), etag: etag}
	response.Header().Set("ETag", etag)
	response.WriteHeader(http.StatusOK)
}

func (s *protocolServer) setFailure(fail func(string, string) int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = fail
}

func (s *protocolServer) corrupt(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, found := s.objects[key]
	if !found {
		return false
	}
	object.data = []byte("corrupt")
	digest := sha256.Sum256(object.data)
	object.etag = `"` + hex.EncodeToString(digest[:]) + `"`
	s.objects[key] = object
	return true
}

func (s *protocolServer) requestSnapshot() []protocolRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocolRequest(nil), s.requests...)
}

func protocolKey(path string) (string, bool) {
	trimmed := strings.TrimPrefix(path, "/")
	bucket, key, found := strings.Cut(trimmed, "/")
	return key, found && bucket == testBucket && key != ""
}

func writeS3Error(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/xml")
	response.WriteHeader(status)
	_, _ = fmt.Fprintf(response, "<Error><Code>%s</Code><Message>request failed</Message></Error>", code)
}

func newTestBackend(t *testing.T, service *protocolServer) (*Backend, *httptest.Server) {
	t.Helper()
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)
	config := aws.Config{
		Region:      "us-east-1",
		Credentials: staticCredentials{},
		HTTPClient:  httpServer.Client(),
		Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
	}
	client := awss3.NewFromConfig(config, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(httpServer.URL)
		options.UsePathStyle = true
	})
	backend, err := New(client, Config{Bucket: testBucket})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return backend, httpServer
}

func TestNewIsInertAndValidatesConfiguration(t *testing.T) {
	var calls atomic.Int64
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, stderrors.New("unexpected request")
	})
	client := awss3.New(awss3.Options{
		BaseEndpoint: aws.String("https://s3.invalid"),
		Credentials:  staticCredentials{},
		HTTPClient:   &http.Client{Transport: transport},
		Region:       "us-east-1",
	})
	if _, err := New(client, Config{Bucket: testBucket}); err != nil {
		t.Fatalf("New: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("constructor network calls = %d, want 0", calls.Load())
	}
	for name, test := range map[string]struct {
		client *awss3.Client
		config Config
	}{
		"nil client":       {config: Config{Bucket: testBucket}},
		"blank bucket":     {client: client},
		"negative maximum": {client: client, config: Config{Bucket: testBucket, MaxObjectBytes: -1}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(test.client, test.config); err == nil {
				t.Fatal("New error = nil")
			}
		})
	}
}

func TestBackendRequiresConditionalWritesAndBoundedObjects(t *testing.T) {
	service := newProtocolServer()
	backend, _ := newTestBackend(t, service)
	if _, err := backend.Put(
		context.Background(), "unconditional", []byte("data"), catalogstore.ObjectPutCondition{},
	); !pkgerrors.IsValidationError(err) {
		t.Fatalf("unconditional Put error = %v, want validation error", err)
	}
	small, err := New(backend.client, Config{Bucket: testBucket, MaxObjectBytes: 3})
	if err != nil {
		t.Fatalf("New small: %v", err)
	}
	if _, err := small.Put(
		context.Background(), "oversized", []byte("four"), catalogstore.ObjectPutCondition{IfAbsent: true},
	); !pkgerrors.IsValidationError(err) {
		t.Fatalf("oversized Put error = %v, want validation error", err)
	}

	service.rejectCondition = true
	_, err = backend.Put(
		context.Background(), "unsupported", []byte("data"), catalogstore.ObjectPutCondition{IfAbsent: true},
	)
	var resourceError *pkgerrors.ResourceError
	if !stderrors.As(err, &resourceError) {
		t.Fatalf("unsupported conditional write error = %v, want *errors.ResourceError", err)
	}
}

func TestBackendUsesExactS3ConditionalHeadersAndTypedErrors(t *testing.T) {
	service := newProtocolServer()
	backend, _ := newTestBackend(t, service)
	ctx := context.Background()

	first, err := backend.Put(ctx, "pointer", []byte("first"), catalogstore.ObjectPutCondition{IfAbsent: true})
	if err != nil {
		t.Fatalf("Put absent: %v", err)
	}
	if first.Version == "" {
		t.Fatal("Put version is empty")
	}
	_, err = backend.Put(ctx, "pointer", []byte("duplicate"), catalogstore.ObjectPutCondition{IfAbsent: true})
	if !pkgerrors.IsConflict(err) {
		t.Fatalf("duplicate Put error = %v, want typed conflict", err)
	}
	second, err := backend.Put(
		ctx, "pointer", []byte("second"), catalogstore.ObjectPutCondition{IfVersion: first.Version},
	)
	if err != nil {
		t.Fatalf("Put version: %v", err)
	}
	_, err = backend.Put(
		ctx, "pointer", []byte("stale"), catalogstore.ObjectPutCondition{IfVersion: first.Version},
	)
	if !pkgerrors.IsConflict(err) {
		t.Fatalf("stale Put error = %v, want typed conflict", err)
	}
	got, err := backend.Get(ctx, "pointer")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got.Data, []byte("second")) || got.Version != second.Version {
		t.Fatalf("Get = %#v, want second bytes/version", got)
	}
	if _, err := backend.Get(ctx, "missing"); !pkgerrors.IsNotFound(err) {
		t.Fatalf("missing Get error = %v, want typed not found", err)
	}

	requests := service.requestSnapshot()
	var foundAbsent, foundVersion bool
	for _, request := range requests {
		foundAbsent = foundAbsent || request.ifNoneMatch == "*"
		foundVersion = foundVersion || request.ifMatch == first.Version
	}
	if !foundAbsent || !foundVersion {
		t.Fatalf("conditional headers: absent=%t version=%t requests=%#v", foundAbsent, foundVersion, requests)
	}
}

func TestS3CatalogStoreConformanceReopenRollbackAndCorruption(t *testing.T) {
	service := newProtocolServer()
	backend, _ := newTestBackend(t, service)
	store := newObjectStore(t, backend, "conformance")
	ctx := context.Background()

	if _, err := store.Current(ctx); !pkgerrors.IsNotFound(err) {
		t.Fatalf("empty Current error = %v, want not found", err)
	}
	first := testGeneration("generation-1", "first")
	if err := store.Commit(ctx, first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}
	if err := store.Commit(ctx, first, ""); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	first.Payload[0] ^= 0xff
	assertCurrent(t, store, testGeneration("generation-1", "first"))
	read, err := store.Get(ctx, "generation-1")
	if err != nil {
		t.Fatalf("Get first: %v", err)
	}
	read.Payload[0] ^= 0xff
	read.Manifest.Validation.Checks[0].Name = "mutated-output"
	assertCurrent(t, store, testGeneration("generation-1", "first"))

	second := testGeneration("generation-2", "second")
	if err := store.Commit(ctx, second, "generation-1"); err != nil {
		t.Fatalf("Commit second: %v", err)
	}
	retained, err := store.Get(ctx, "generation-1")
	if err != nil {
		t.Fatalf("Get retained: %v", err)
	}
	if diff := cmp.Diff(testGeneration("generation-1", "first"), retained); diff != "" {
		t.Fatalf("retained generation mismatch (-want +got):\n%s", diff)
	}
	stale := testGeneration("generation-stale", "stale")
	if err := store.Commit(ctx, stale, "generation-1"); !pkgerrors.IsConflict(err) {
		t.Fatalf("stale Commit error = %v, want conflict", err)
	}
	if _, err := store.Get(ctx, "generation-stale"); !pkgerrors.IsNotFound(err) {
		t.Fatalf("stale candidate Get error = %v, want not found", err)
	}
	invalid := testGeneration("generation-invalid", "invalid")
	invalid.Payload = append(invalid.Payload, 'x')
	if err := store.Commit(ctx, invalid, "generation-2"); !pkgerrors.IsValidationError(err) {
		t.Fatalf("invalid Commit error = %v, want validation error", err)
	}
	collision := testGeneration("generation-2", "different")
	if err := store.Commit(ctx, collision, "generation-2"); !pkgerrors.IsConflict(err) {
		t.Fatalf("identity collision error = %v, want conflict", err)
	}

	reopened := newObjectStore(t, backend, "conformance")
	assertCurrent(t, reopened, second)
	if err := reopened.Commit(ctx, testGeneration("generation-1", "first"), "generation-2"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	assertCurrent(t, reopened, testGeneration("generation-1", "first"))

	if !service.corrupt(generationObjectKey("conformance", "generation-1", "catalog.json")) {
		t.Fatal("payload object not found for corruption")
	}
	if _, err := reopened.Get(ctx, "generation-1"); !pkgerrors.IsValidationError(err) {
		t.Fatalf("corrupt Get error = %v, want validation error", err)
	}
}

func TestS3CatalogStoreConcurrentCAS(t *testing.T) {
	service := newProtocolServer()
	backend, _ := newTestBackend(t, service)
	firstStore := newObjectStore(t, backend, "concurrent")
	secondStore := newObjectStore(t, backend, "concurrent")
	base := testGeneration("cas-base", "base")
	if err := firstStore.Commit(context.Background(), base, ""); err != nil {
		t.Fatalf("Commit base: %v", err)
	}
	stores := []*catalogstore.Object{firstStore, secondStore}
	candidates := []catalogstore.Generation{
		testGeneration("cas-left", "left"),
		testGeneration("cas-right", "right"),
	}
	start := make(chan struct{})
	results := make(chan error, len(stores))
	var wait sync.WaitGroup
	for index := range stores {
		wait.Go(func() {
			<-start
			results <- stores[index].Commit(context.Background(), candidates[index], "cas-base")
		})
	}
	close(start)
	wait.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case pkgerrors.IsConflict(err):
			conflicts++
		default:
			t.Fatalf("Commit error = %v, want nil or typed conflict", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("CAS results: successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
}

func TestS3CatalogStoreUploadAndPromotionFaultsPreserveCurrent(t *testing.T) {
	service := newProtocolServer()
	backend, _ := newTestBackend(t, service)
	store := newObjectStore(t, backend, "faults")
	first := testGeneration("fault-first", "first")
	if err := store.Commit(context.Background(), first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}

	service.setFailure(func(method, key string) int {
		if method == http.MethodPut && strings.HasSuffix(key, "/catalog.json") {
			return http.StatusServiceUnavailable
		}
		return 0
	})
	second := testGeneration("fault-second", "second")
	if err := store.Commit(context.Background(), second, "fault-first"); err == nil {
		t.Fatal("payload fault Commit error = nil")
	}
	assertCurrent(t, store, first)
	if _, err := store.Get(context.Background(), "fault-second"); err == nil {
		t.Fatal("incomplete candidate is readable")
	}

	service.setFailure(nil)
	if err := store.Commit(context.Background(), second, "fault-first"); err != nil {
		t.Fatalf("retry second: %v", err)
	}
	service.setFailure(func(method, key string) int {
		if method == http.MethodPut && strings.HasSuffix(key, "/current.json") {
			return http.StatusServiceUnavailable
		}
		return 0
	})
	third := testGeneration("fault-third", "third")
	if err := store.Commit(context.Background(), third, "fault-second"); err == nil {
		t.Fatal("promotion fault Commit error = nil")
	}
	assertCurrent(t, store, second)
	staged, err := store.Get(context.Background(), "fault-third")
	if err != nil {
		t.Fatalf("Get complete staged generation: %v", err)
	}
	if diff := cmp.Diff(third, staged); diff != "" {
		t.Fatalf("staged generation mismatch (-want +got):\n%s", diff)
	}
}

func newObjectStore(t *testing.T, backend catalogstore.ObjectBackend, prefix string) *catalogstore.Object {
	t.Helper()
	store, err := catalogstore.NewObject(backend, prefix)
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	return store
}

func assertCurrent(t *testing.T, store catalogstore.Store, want catalogstore.Generation) {
	t.Helper()
	got, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Current mismatch (-want +got):\n%s", diff)
	}
}

func testGeneration(id, value string) catalogstore.Generation {
	payload := fmt.Appendf(nil, `{"value":%q}`, value)
	evidence := catalogs.DescribeCatalogPayload([]byte("evidence:" + value))
	generatedAt := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	return catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    id,
			GeneratedAt:     generatedAt,
			Payload:         catalogs.DescribeCatalogPayload(payload),
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "catalog-validator/v1",
				ValidatedAt:      generatedAt.Add(time.Second),
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{
					{Name: "schema", Status: catalogs.GenerationValidationCheckPassed},
				},
			},
			SyncRunID: "sync-" + id,
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.ProvidersID,
					ObservationID: "observation-" + id,
					ObservedAt:    generatedAt,
					Revision: catalogmeta.ObservationRevision{
						Kind:  catalogmeta.ObservationRevisionKindContentDigest,
						Value: evidence.Checksum,
					},
					Completeness:     catalogmeta.ObservationCompletenessComplete,
					Status:           catalogmeta.ObservationStatusSucceeded,
					EvidenceChecksum: evidence.Checksum,
				},
			},
			ReviewCandidates: []catalogmeta.ReviewCandidate{},
			Completeness:     catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
}

func generationObjectKey(prefix, id, filename string) string {
	digest := sha256.Sum256([]byte(id))
	return prefix + "/generations/" + hex.EncodeToString(digest[:]) + "/" + filename
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
