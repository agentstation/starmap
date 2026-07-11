package starmap

import (
	"testing"
	"time"
)

func TestEmbeddedBudgetReadinessReportsGenerationAgeVersionAndSize(t *testing.T) {
	client, err := New(
		WithEmbeddedBootstrapMaxAge(24*time.Hour),
		WithEmbeddedBootstrapMaxSizeBytes(2<<20),
	)
	if err != nil {
		t.Fatalf("New offline bootstrap: %v", err)
	}
	generatedAt := client.embeddedBootstrap.GeneratedAt
	client.now = func() time.Time { return generatedAt.Add(12 * time.Hour) }

	readiness := client.Readiness()
	if !readiness.Ready || len(readiness.Issues) != 0 {
		t.Fatalf("readiness = %#v", readiness)
	}
	info := readiness.Embedded
	if !info.Active || info.GenerationID == "" || info.ManifestVersion == 0 ||
		info.SchemaVersion == 0 || info.PayloadChecksum == "" || info.PayloadSizeBytes <= 0 ||
		info.AgeSeconds != int64((12*time.Hour)/time.Second) {
		t.Fatalf("embedded bootstrap info = %#v", info)
	}
}

func TestEmbeddedBudgetReadinessFailsClosedForStaleAndOversizeBootstrap(t *testing.T) {
	client, err := New(
		WithEmbeddedBootstrapMaxAge(time.Hour),
		WithEmbeddedBootstrapMaxSizeBytes(1),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	client.now = func() time.Time { return client.embeddedBootstrap.GeneratedAt.Add(2 * time.Hour) }

	readiness := client.Readiness()
	if readiness.Ready || len(readiness.Issues) != 2 {
		t.Fatalf("readiness = %#v, want stale and oversize", readiness)
	}
	if readiness.Issues[0].Code != ReadinessIssueEmbeddedBootstrapStale ||
		readiness.Issues[1].Code != ReadinessIssueEmbeddedBootstrapOversize {
		t.Fatalf("readiness issue codes = %#v", readiness.Issues)
	}

	client.swapCatalogGeneration(client.Catalog(), "published-generation")
	readiness = client.Readiness()
	if !readiness.Ready || readiness.Embedded.Active {
		t.Fatalf("published generation should supersede bootstrap budgets: %#v", readiness)
	}
}

func TestEmbeddedBudgetOptionsRejectNonPositiveValues(t *testing.T) {
	for _, option := range []Option{
		WithEmbeddedBootstrapMaxAge(0),
		WithEmbeddedBootstrapMaxSizeBytes(0),
	} {
		if _, err := New(option); err == nil {
			t.Fatal("New accepted a non-positive embedded bootstrap budget")
		}
	}
}
