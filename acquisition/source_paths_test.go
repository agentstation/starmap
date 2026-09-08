package acquisition

import (
	"bytes"
	stderrors "errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type sourcePathTransport struct {
	testing *testing.T
	payload []byte
	calls   int
}

func (transport *sourcePathTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.String() != "https://models.dev/api.json" {
		transport.testing.Fatalf("unexpected request: %s", request.URL)
	}
	transport.calls++
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(transport.payload)), Request: request}, nil
}

func TestSyncUsesHostSourceDirectoriesAndExplicitOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	payload, err := embedded.FS.ReadFile("sources/models.dev/api.json")
	if err != nil {
		t.Fatal(err)
	}
	transport := &sourcePathTransport{testing: t, payload: payload}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "host", true: "explicit"}[explicit], func(t *testing.T) {
			root := t.TempDir()
			directories, err := productpaths.SourceDirectoriesAt(filepath.Join(root, "starport-cache"))
			if err != nil {
				t.Fatal(err)
			}
			client, err := starmap.New()
			if err != nil {
				t.Fatal(err)
			}
			syncer, err := New(client, WithSourceDirectories(directories))
			if err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("constructor created source storage")
			}
			opts := []pkgsync.Option{pkgsync.WithDryRun(true), pkgsync.WithSources(sources.ModelsDevHTTPID)}
			selected := directories.Cache
			if explicit {
				selected = filepath.Join(root, "explicit")
				opts = append(opts, pkgsync.WithSourcesDir(selected))
			}
			if _, err := syncer.Sync(t.Context(), opts...); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"api.json", "api.json.metadata.json"} {
				if _, err := os.Stat(filepath.Join(selected, "models.dev", name)); err != nil {
					t.Fatal("source wrote outside selected directory", err)
				}
			}
			if explicit {
				if _, err := os.Stat(directories.Cache); !os.IsNotExist(err) {
					t.Fatal("explicit source selection also created default cache")
				}
			}
		})
	}
	if transport.calls != 2 {
		t.Fatalf("HTTP observations = %d", transport.calls)
	}
	if _, err := os.Stat(filepath.Join(home, ".starmap")); !os.IsNotExist(err) {
		t.Fatal("source touched legacy product root")
	}
}

func TestWorkspaceProjectionUsesSelectedCheckoutLogos(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	directories, err := productpaths.SourceDirectoriesAt(filepath.Join(root, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"provider", "author"} {
		dir := filepath.Join(directories.Checkouts, "models.dev-git", "providers", id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "logo.svg"), []byte("<svg>"+id+"</svg>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	catalog := acquisitionTestCatalog(t)
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "workspace")
	result := projectCommittedCatalog(t.Context(), catalog, output, starmap.Publication{Published: true, GenerationID: "selected-logo-source", PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum}, workspace.InputExpectation{}, &pkgsync.Options{SourceDirectories: directories})
	if result.Status != pkgsync.ProjectionStatusApplied {
		t.Fatalf("projection = %+v", result)
	}
	for _, item := range []struct{ kind, id string }{{"providers", "provider"}, {"authors", "author"}} {
		encoded, err := os.ReadFile(filepath.Join(output, item.kind, item.id, "logo.svg"))
		if err != nil || string(encoded) != "<svg>"+item.id+"</svg>" {
			t.Fatalf("selected logo %s = %q, %v", item.id, encoded, err)
		}
	}
}

func TestImportReleaseRefusesSourceWorkspaceOverlapBeforePublication(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	directories, err := productpaths.SourceDirectoriesAt(root)
	if err != nil {
		t.Fatal(err)
	}
	client, err := starmap.New(starmap.WithCatalogStore(storage.NewMemory()), starmap.WithCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	syncer, err := New(client, WithSourceDirectories(directories))
	if err != nil {
		t.Fatal(err)
	}
	before := client.Catalog()
	catalog := buildImportCatalog(t, importCatalogBuilder(t, "Release Model Name", "Release Description", true, false))
	release := importReleaseFixture(t, catalog)
	_, err = syncer.ImportRelease(t.Context(), release, &importPublisherVerifier{})
	var configError *pkgerrors.ConfigError
	if !stderrors.As(err, &configError) || configError.Component != "catalog filesystem layout" {
		t.Fatalf("source/workspace overlap did not refuse before publication: %v", err)
	}
	if client.Catalog() != before {
		t.Fatal("refused import published a catalog")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused import wrote workspace files")
	}
}
