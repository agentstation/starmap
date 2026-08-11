package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/remote"
	"github.com/agentstation/starmap/server"
)

const testBucket = "starmap-catalogs"

type credentials struct{}

func (credentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Source:          "test",
	}, nil
}

type object struct {
	data []byte
	etag string
}

type objectService struct {
	mu      sync.Mutex
	objects map[string]object
	calls   atomic.Int64
}

func newObjectService() *objectService {
	return &objectService{objects: make(map[string]object)}
}

func (s *objectService) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	s.calls.Add(1)
	key, found := objectKey(request.URL.Path)
	if !found {
		writeError(response, http.StatusNotFound, "NoSuchBucket")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch request.Method {
	case http.MethodGet:
		value, exists := s.objects[key]
		if !exists {
			writeError(response, http.StatusNotFound, "NoSuchKey")
			return
		}
		response.Header().Set("ETag", value.etag)
		response.Header().Set("Content-Length", fmt.Sprint(len(value.data)))
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(value.data)
	case http.MethodPut:
		s.put(response, request, key)
	default:
		writeError(response, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func (s *objectService) put(response http.ResponseWriter, request *http.Request, key string) {
	current, exists := s.objects[key]
	switch {
	case request.Header.Get("If-None-Match") == "*" && exists:
		writeError(response, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	case request.Header.Get("If-Match") != "" &&
		(!exists || request.Header.Get("If-Match") != current.etag):
		writeError(response, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	case request.Header.Get("If-None-Match") == "" && request.Header.Get("If-Match") == "":
		writeError(response, http.StatusBadRequest, "ConditionalWriteRequired")
		return
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		writeError(response, http.StatusBadRequest, "InvalidBody")
		return
	}
	digest := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	s.objects[key] = object{data: append([]byte(nil), data...), etag: etag}
	response.Header().Set("ETag", etag)
	response.WriteHeader(http.StatusOK)
}

func objectKey(path string) (string, bool) {
	bucket, key, found := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	return key, found && bucket == testBucket && key != ""
}

func writeError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/xml")
	response.WriteHeader(status)
	_, _ = fmt.Fprintf(response, "<Error><Code>%s</Code><Message>failed</Message></Error>", code)
}

func newS3Client(serviceURL string, httpClient *http.Client) *awss3.Client {
	config := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials{},
		HTTPClient:  httpClient,
		Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
	}
	return awss3.NewFromConfig(config, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(serviceURL)
		options.UsePathStyle = true
	})
}

func TestStorageConfigRejectsAmbiguousOrIncompleteSelection(t *testing.T) {
	client := awss3.New(awss3.Options{
		BaseEndpoint: aws.String("https://s3.invalid"),
		Credentials:  credentials{},
		HTTPClient:   http.DefaultClient,
		Region:       "us-east-1",
	})
	for name, config := range map[string]StorageConfig{
		"missing mode": {},
		"filesystem path missing": {
			Mode: StorageFilesystem,
		},
		"filesystem with object field": {
			Mode: StorageFilesystem, FilesystemPath: t.TempDir(), S3Bucket: testBucket,
		},
		"object client missing": {
			Mode: StorageObject, S3Bucket: testBucket, ObjectPrefix: "production",
		},
		"object bucket missing": {
			Mode: StorageObject, S3Client: client, ObjectPrefix: "production",
		},
		"object prefix missing": {
			Mode: StorageObject, S3Client: client, S3Bucket: testBucket,
		},
		"object with filesystem field": {
			Mode: StorageObject, FilesystemPath: t.TempDir(),
			S3Client: client, S3Bucket: testBucket, ObjectPrefix: "production",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := config.Open(); err == nil {
				t.Fatal("Open error = nil")
			}
		})
	}
}

func TestFilesystemAndObjectServerStorageAreReactiveAndRestartable(t *testing.T) {
	service := newObjectService()
	httpServer := httptest.NewServer(service)
	defer httpServer.Close()
	s3Client := newS3Client(httpServer.URL, httpServer.Client())

	configs := map[string]StorageConfig{
		"filesystem": {
			Mode:           StorageFilesystem,
			FilesystemPath: t.TempDir(),
		},
		"object": {
			Mode:         StorageObject,
			S3Client:     s3Client,
			S3Bucket:     testBucket,
			ObjectPrefix: "production",
		},
	}
	for name, storageConfig := range configs {
		t.Run(name, func(t *testing.T) {
			beforeOpenCalls := service.calls.Load()
			store, err := storageConfig.Open()
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if service.calls.Load() != beforeOpenCalls {
				t.Fatal("storage construction performed an object-network request")
			}
			runStorageDrill(t, storageConfig, store, service.calls.Load)
		})
	}
}

func runStorageDrill(
	t *testing.T,
	storageConfig StorageConfig,
	store catalogstore.Store,
	networkCalls func() int64,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bootstrap, err := starmap.NewContext(ctx)
	if err != nil {
		t.Fatalf("bootstrap NewContext: %v", err)
	}
	generation, err := bootstrap.CurrentGeneration(ctx)
	if err != nil {
		t.Fatalf("bootstrap CurrentGeneration: %v", err)
	}
	if err := store.Commit(ctx, generation, ""); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	client, err := starmap.NewContext(ctx, starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("stored NewContext: %v", err)
	}
	if client.CurrentGenerationID() != generation.Manifest.GenerationID {
		t.Fatalf(
			"initial generation = %q, want %q",
			client.CurrentGenerationID(),
			generation.Manifest.GenerationID,
		)
	}
	settledNetworkCalls := networkCalls()
	time.Sleep(50 * time.Millisecond)
	if networkCalls() != settledNetworkCalls {
		t.Fatal("starmap.NewContext left a hidden object-network lifecycle running")
	}

	config := server.DefaultConfig()
	config.RateLimit = 0
	config.MetricsEnabled = false
	config.SSEHeartbeatInterval = 25 * time.Millisecond
	config.SSEWriteTimeout = time.Second
	starmapServer, err := server.New(client, config)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := starmapServer.Start(); err != nil {
		t.Fatalf("server.Start: %v", err)
	}
	publisher := httptest.NewServer(starmapServer.Handler())

	subscriber, err := remote.New(remote.Config{
		BaseURL:                   publisher.URL + "/api/v1",
		CatalogStore:              catalogstore.NewMemory(),
		ExpectedHeartbeatInterval: 25 * time.Millisecond,
		LivenessTimeout:           time.Second,
	})
	if err != nil {
		t.Fatalf("remote.New: %v", err)
	}
	if err := subscriber.Start(ctx); err != nil {
		t.Fatalf("subscriber.Start: %v", err)
	}

	publication, err := client.Update(ctx, func(
		_ context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		builder, err := catalogs.NewBuilderFrom(current)
		if err != nil {
			return nil, err
		}
		if err := builder.SetProvider(catalogs.Provider{
			ID:   "storage-drill",
			Name: "Storage Drill",
		}); err != nil {
			return nil, err
		}
		candidate, err := builder.Build()
		if err != nil {
			return nil, err
		}
		return starmap.NewCandidate(candidate, starmap.CandidateEvidence{})
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !publication.Published {
		t.Fatal("Update did not publish")
	}
	eventually(t, 5*time.Second, func() bool {
		return subscriber.Health().ActiveGenerationID == publication.GenerationID
	})
	if _, err := subscriber.Catalog().Provider("storage-drill"); err != nil {
		t.Fatalf("subscriber catalog provider: %v", err)
	}

	if err := subscriber.Close(); err != nil {
		t.Errorf("subscriber.Close: %v", err)
	}
	publisher.Close()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := starmapServer.Shutdown(shutdownCtx); err != nil {
		t.Errorf("server.Shutdown: %v", err)
	}

	reopenedStore, err := storageConfig.Open()
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	restarted, err := starmap.NewContext(ctx, starmap.WithCatalogStore(reopenedStore))
	if err != nil {
		t.Fatalf("restart NewContext: %v", err)
	}
	if restarted.CurrentGenerationID() != publication.GenerationID {
		t.Fatalf(
			"restarted generation = %q, want %q",
			restarted.CurrentGenerationID(),
			publication.GenerationID,
		)
	}
	if _, err := restarted.Catalog().Provider("storage-drill"); err != nil {
		t.Fatalf("restarted catalog provider: %v", err)
	}
}

func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}
