package valkey

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	stores3 "github.com/agentstation/starmap/pkg/catalogs/storage/s3"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestCoordinatedCatalogSurvivesProcessAndServiceRestart(t *testing.T) {
	coordinationContainer := os.Getenv("STARMAP_TEST_COORDINATION_CONTAINER")
	objectContainer := os.Getenv("STARMAP_TEST_OBJECT_CONTAINER")
	endpoint := os.Getenv("STARMAP_TEST_S3_ENDPOINT")
	if os.Getenv("STARMAP_TEST_COORDINATION_SERVER_CHANGES") != "1" || coordinationContainer == "" || objectContainer == "" || endpoint == "" {
		t.Skip("restart qualification requires isolated coordination and object containers")
	}
	objects, bucket := nativeObjectFixture(t, endpoint)
	namespace := rand.Text()
	first := nativeCoordinatedStore(t, objects, "parent", namespace)
	if err := first.Commit(t.Context(), nativeCatalogGeneration("old"), ""); err != nil {
		t.Fatal(err)
	}
	if err := first.Commit(t.Context(), nativeCatalogGeneration("current"), "old"); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"reader", "writer"} {
		runCatalogProcess(t, role, namespace, bucket)
	}
	claims, err := first.ReaderClaims(t.Context())
	if err != nil || len(claims.Claims) != 1 || claims.Claims[0].OwnerID != "exited-reader" {
		t.Fatalf("exited process lost its durable claim: %+v, %v", claims, err)
	}
	for _, container := range []string{coordinationContainer, objectContainer} {
		if output, err := exec.CommandContext(t.Context(), "docker", "restart", container).CombinedOutput(); err != nil {
			t.Fatalf("restart isolated catalog service: %v: %s", err, output)
		}
	}
	waitForRestartedService(t)
	waitForObjectService(t, objects)
	reopened := nativeCoordinatedStore(t, objects, "replacement", namespace)
	current, err := reopened.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != "current" {
		t.Fatalf("restart lost current catalog: %+v, %v", current.Manifest, err)
	}
	claims, err = reopened.ReaderClaims(t.Context())
	if err != nil || len(claims.Claims) != 1 || claims.Claims[0].GenerationID != "old" {
		t.Fatalf("restart lost reader protection: %+v, %v", claims, err)
	}
	request := storage.RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20}
	report, err := reopened.Collect(t.Context(), request)
	if err != nil || !report.OverLimit || len(report.Removed) != 0 {
		t.Fatalf("restart allowed collection of protected content: %+v, %v", report, err)
	}
	claims, err = reopened.ReaderClaims(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// The child process exited before the parent requests owner recovery.
	if err := reopened.ReleaseFencedOwner(t.Context(), "exited-reader", claims.Revision); err != nil {
		t.Fatal(err)
	}
	_, connection := testBackend(t)
	coordination, err := New(connection, Config{Prefix: "starmap-native-catalog:" + namespace})
	if err != nil {
		t.Fatal(err)
	}
	collector, err := storage.NewCoordinatedObject(objects, coordination, storage.CoordinatedObjectConfig{
		Prefix: "catalog", CoordinationKey: "catalog", OwnerID: "collector",
		Now: func() time.Time { return time.Now().Add(2 * storage.DefaultObjectWriteLifetime) },
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err = collector.Collect(t.Context(), request)
	if err != nil || len(report.Removed) != 1 || report.Removed[0] != "old" {
		t.Fatalf("fenced reader recovery failed: %+v, %v", report, err)
	}
	page, err := objects.List(t.Context(), storage.ObjectListRequest{Prefix: "catalog/uploads/", Limit: 10})
	if err != nil || len(page.Objects) != 1 {
		t.Fatalf("expired process upload survived recovery: %+v, %v", page, err)
	}
	if err := reopened.Commit(t.Context(), nativeCatalogGeneration("replacement"), "current"); err != nil {
		t.Fatalf("restart prevented subsequent publication: %v", err)
	}
}

func runCatalogProcess(t *testing.T, role, namespace, bucket string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestCoordinatedCatalogProcess$", "-test.timeout=30s")
	command.Env = append(os.Environ(), "STARMAP_TEST_CATALOG_PROCESS="+role, "STARMAP_TEST_CATALOG_NAMESPACE="+namespace, "STARMAP_TEST_CATALOG_BUCKET="+bucket)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("isolated catalog process failed: %v: %s", err, output)
	}
}

func TestCoordinatedCatalogProcess(t *testing.T) {
	role := os.Getenv("STARMAP_TEST_CATALOG_PROCESS")
	if role == "" {
		t.Skip("catalog process fixture runs only from its parent test")
	}
	objects, err := stores3.New(nativeObjectConnection(t, os.Getenv("STARMAP_TEST_S3_ENDPOINT")), stores3.Config{
		Bucket: os.Getenv("STARMAP_TEST_CATALOG_BUCKET"), MaxObjectBytes: storage.MaxCoordinatedObjectBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	namespace := os.Getenv("STARMAP_TEST_CATALOG_NAMESPACE")
	if role == "reader" {
		store := nativeCoordinatedStore(t, objects, "exited-reader", namespace)
		if _, _, err := store.AcquireGeneration(t.Context(), "old"); err != nil {
			t.Fatal(err)
		}
		// Exit without releasing the claim or running process cleanup.
		os.Exit(0)
	}
	if role != "writer" {
		t.Fatal("unknown catalog process role")
	}
	_, connection := testBackend(t)
	coordination, err := New(connection, Config{Prefix: "starmap-native-catalog:" + namespace})
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewCoordinatedObject(exitAfterObjectUpload{objects}, coordination, storage.CoordinatedObjectConfig{
		Prefix: "catalog", CoordinationKey: "catalog", OwnerID: "exited-writer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), nativeCatalogGeneration("unfinished"), "current"); err != nil {
		t.Fatal(err)
	}
	t.Fatal("writer process did not exit after uploading bytes")
}

type exitAfterObjectUpload struct{ *stores3.Backend }

func (b exitAfterObjectUpload) Put(ctx context.Context, key string, data []byte, condition storage.ObjectPutCondition) (storage.ObjectValue, error) {
	value, err := b.Backend.Put(ctx, key, data, condition)
	if err == nil && strings.Contains(key, "/uploads/") {
		os.Exit(0)
	}
	return value, err
}

func waitForObjectService(t *testing.T, objects *stores3.Backend) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, err := objects.Get(ctx, "catalog/current.json")
		if err == nil {
			return
		}
		if errors.IsNotFound(err) {
			t.Fatal("object restart lost the catalog binding")
		}
		select {
		case <-ctx.Done():
			t.Fatalf("object service did not recover: %v", err)
		case <-ticker.C:
		}
	}
}
