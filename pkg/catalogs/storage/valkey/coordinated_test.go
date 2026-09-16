package valkey

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	stores3 "github.com/agentstation/starmap/pkg/catalogs/storage/s3"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestCoordinatedCatalogUsesNativeObjectAndCoordinationServices(t *testing.T) {
	endpoint := os.Getenv("STARMAP_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("STARMAP_TEST_S3_ENDPOINT selects an isolated local object service")
	}
	objects := nativeObjectBackend(t, endpoint)
	namespace := rand.Text()
	first := nativeCoordinatedStore(t, objects, "first", namespace)
	second := nativeCoordinatedStore(t, objects, "second", namespace)
	expected := ""
	for _, id := range []string{"baseline", "old", "current"} {
		if err := first.Commit(t.Context(), nativeCatalogGeneration(id), expected); err != nil {
			t.Fatal(err)
		}
		expected = id
	}
	held, release, err := second.AcquireGeneration(t.Context(), "old")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	request := storage.RetentionRequest{ExpectedGenerationID: "current", RequiredGenerationIDs: []string{"baseline"}, MaxGenerations: 2, MaxBytes: 1 << 20}
	report, err := first.Collect(t.Context(), request)
	if err != nil || !report.OverLimit || len(report.Removed) != 0 {
		t.Fatalf("native collection lost reader protection: %+v, %v", report, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	report, err = first.Collect(t.Context(), request)
	if err != nil || len(report.Removed) != 1 || report.Removed[0] != "old" {
		t.Fatalf("native collection did not remove released content: %+v, %v", report, err)
	}
	if err := held.Validate(); err != nil {
		t.Fatalf("native collection changed caller-owned bytes: %v", err)
	}
	current, err := second.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != "current" {
		t.Fatalf("independent native client lost current: %+v, %v", current.Manifest, err)
	}
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for index, selected := range []*storage.CoordinatedObject{first, second} {
		workers.Go(func() {
			results <- selected.Commit(t.Context(), nativeCatalogGeneration(fmt.Sprintf("candidate-%d", index)), "current")
		})
	}
	workers.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.IsConflict(err) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("native publication admitted %d writers and refused %d", successes, conflicts)
	}
	current, err = second.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	capacity := nativeCatalogGeneration("capacity")
	capacity.Payload = fmt.Appendf(nil, `{"value":%q}`, strings.Repeat("x", storage.MaxFilesystemPayloadBytes-64))
	capacity.Manifest.Payload = catalogs.DescribeCatalogPayload(capacity.Payload)
	if err := first.Commit(t.Context(), capacity, current.Manifest.GenerationID); err != nil {
		t.Fatalf("native catalog capacity publication failed: %v", err)
	}
	retained, err := second.Current(t.Context())
	if err != nil || !bytes.Equal(retained.Payload, capacity.Payload) {
		t.Fatalf("native catalog capacity read failed: %v", err)
	}
	t.Logf("native object capacity: %d payload bytes", len(capacity.Payload))
}

func nativeCoordinatedStore(t *testing.T, objects *stores3.Backend, owner, namespace string) *storage.CoordinatedObject {
	t.Helper()
	_, connection := testBackend(t)
	coordination, err := New(connection, Config{Prefix: "starmap-native-catalog:" + namespace})
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewCoordinatedObject(objects, coordination, storage.CoordinatedObjectConfig{Prefix: "catalog", CoordinationKey: "catalog", OwnerID: owner})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func nativeObjectBackend(t *testing.T, endpoint string) *stores3.Backend {
	t.Helper()
	objects, _ := nativeObjectFixture(t, endpoint)
	return objects
}

func nativeObjectConnection(t *testing.T, endpoint string) *awss3.Client {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		t.Fatal("native qualification requires an explicit loopback HTTP endpoint")
	}
	return awss3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("starmap-test", "starmap-test-secret-only", ""), HTTPClient: &http.Client{Timeout: 10 * time.Second}}, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
		options.RetryMaxAttempts = 1
	})
}

func nativeObjectFixture(t *testing.T, endpoint string) (*stores3.Backend, string) {
	t.Helper()
	connection := nativeObjectConnection(t, endpoint)
	bucket := "starmap-" + strings.ToLower(rand.Text())
	if _, err := connection.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	objects, err := stores3.New(connection, stores3.Config{Bucket: bucket, MaxObjectBytes: storage.MaxCoordinatedObjectBytes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		page, err := objects.List(ctx, storage.ObjectListRequest{Prefix: "catalog/", Limit: 100})
		if err != nil {
			t.Error(err)
			return
		}
		if page.Next != "" {
			t.Error("native fixture exceeds its cleanup bound")
			return
		}
		for _, entry := range page.Objects {
			if err := objects.Delete(ctx, entry.Key, entry.Version); err != nil {
				t.Error(err)
				return
			}
		}
		if _, err := connection.DeleteBucket(ctx, &awss3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Error(err)
		}
	})
	return objects, bucket
}

func nativeCatalogGeneration(id string) catalogs.Generation {
	payload := fmt.Appendf(nil, `{"value":%q}`, id)
	descriptor := catalogs.DescribeCatalogPayload([]byte("evidence:" + id))
	observed := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	return catalogs.Generation{Manifest: catalogs.GenerationManifest{
		ManifestVersion: catalogs.CurrentGenerationManifestVersion, SchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		GenerationID: id, GeneratedAt: observed, Payload: catalogs.DescribeCatalogPayload(payload),
		Validation:         catalogs.GenerationValidationReport{ValidatorVersion: "catalog-validator/v1", ValidatedAt: observed.Add(time.Second), Status: catalogs.GenerationValidationPassed, Checks: []catalogs.GenerationValidationCheck{{Name: "schema", Status: catalogs.GenerationValidationCheckPassed}}},
		SyncRunID:          "sync-" + id,
		SourceObservations: []catalogs.SourceObservationLink{{Source: evidence.ProvidersID, ObservationID: "observation-" + id, ObservedAt: observed, Revision: evidence.ObservationRevision{Kind: evidence.ObservationRevisionKindContentDigest, Value: descriptor.Checksum}, Completeness: evidence.ObservationCompletenessComplete, Status: evidence.ObservationStatusSucceeded, EvidenceChecksum: descriptor.Checksum}},
		ReviewCandidates:   []evidence.ReviewCandidate{}, Completeness: catalogs.GenerationCompletenessComplete,
		ConsumerCompatibility: catalogs.ConsumerCompatibility{MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion, MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion},
	}, Payload: payload}
}
