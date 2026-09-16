package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/server"
	"github.com/agentstation/starmap/server/administration"
)

func TestConfigurationReportsCLIAndAdminParityColdAuthority(t *testing.T) {
	clearCatalogEnvironment(t)
	root := t.TempDir()
	t.Setenv("STARMAP_HOME", root)
	t.Setenv(catalogconfig.Source, "starmap")
	t.Setenv(catalogconfig.SourceURL, "https://catalog.example.test")
	t.Setenv(catalogconfig.SourceStartupPolicy, "require_authority")
	t.Setenv(catalogconfig.SourceAuthorityID, "enterprise")
	t.Setenv(catalogconfig.SourcePolicyID, "production")
	t.Setenv(catalogconfig.SourceAPIKey, "synthetic-upstream-credential")
	t.Setenv(catalogconfig.AcquisitionEnabled, "false")
	app := NewForCommand("test", "test", "test", "test")
	command := app.createRootCommand()
	var cli bytes.Buffer
	command.SetOut(&cli)
	command.SetErr(&cli)
	command.SetArgs([]string{"config", "effective", "--format", "json"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if app.runtime != nil || app.starmap != nil {
		t.Fatal("configuration report initialized a catalog")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("configuration report created product state")
	}
	if bytes.Contains(cli.Bytes(), []byte("synthetic-upstream-credential")) {
		t.Fatal("configuration report exposed a transport credential")
	}
	if !bytes.Contains(cli.Bytes(), []byte(`"presence":"value"`)) || !bytes.Contains(cli.Bytes(), []byte(`"value":"false"`)) || !bytes.Contains(cli.Bytes(), []byte(`"origin":"environment"`)) {
		t.Fatal("report omitted explicit values or their origins")
	}
	administrationConfig, err := app.AdministrationConfig()
	if err != nil || administrationConfig.Audience != "enterprise" {
		t.Fatalf("internal authority audience=%q, error=%v", administrationConfig.Audience, err)
	}
	reports, err := app.ConfigurationReports()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(cli.Bytes()), reports.Effective()) {
		t.Fatal("CLI report differs from its canonical report")
	}
	var schemaCLI bytes.Buffer
	schemaCommand := app.createRootCommand()
	schemaCommand.SetOut(&schemaCLI)
	schemaCommand.SetErr(&schemaCLI)
	schemaCommand.SetArgs([]string{"config", "schema", "--format", "json"})
	if err := schemaCommand.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(schemaCLI.Bytes()), reports.Schema()) {
		t.Fatal("CLI schema differs from its canonical report")
	}
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	manager, adminToken, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "enterprise"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	actor, _ := manager.Authenticate(adminToken, "enterprise")
	subscriberToken, err := manager.Create(t.Context(), actor, "gateway", administration.Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	cfg := server.DefaultConfig()
	cfg.RateLimit = 0
	srv, err := server.New(client, cfg, server.WithAdministration(manager, "enterprise"), server.WithConfigurationReports(reports))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"schema", "effective"} {
		t.Run(name, func(t *testing.T) {
			for _, test := range []struct {
				name, token string
				status      int
			}{{"administrator", adminToken, 200}, {"subscriber", subscriberToken, 403}, {"gateway inference", "starport-inference-key", 401}, {"anonymous", "", 401}} {
				t.Run(test.name, func(t *testing.T) {
					request := httptest.NewRequest(http.MethodGet, "/admin/config/"+name, nil)
					request.Header.Set("Authorization", "Bearer "+test.token)
					response := httptest.NewRecorder()
					srv.Handler().ServeHTTP(response, request)
					if response.Code != test.status {
						t.Fatalf("status=%d want=%d", response.Code, test.status)
					}
					if test.status == 200 {
						want := reports.Schema()
						if name == "effective" {
							want = reports.Effective()
						}
						if !bytes.Equal(bytes.TrimSpace(response.Body.Bytes()), want) {
							t.Fatal("administrative API report differs from canonical CLI content")
						}
						if response.Header().Get("ETag") == "" || response.Header().Get("Cache-Control") != "private, no-store" {
							t.Fatal("configuration response has no private cache policy or revision")
						}
					}
				})
			}
		})
	}
}

func TestConfigurationReportRedactsURLCredentials(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv(catalogconfig.Source, "starmap")
	t.Setenv(catalogconfig.SourceURL, "https://user:synthetic-password@catalog.example.test/catalog?token=synthetic-query#synthetic-fragment")
	app := NewForCommand("test", "test", "test", "test")
	command := app.createRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"config", "effective"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"synthetic-password", "synthetic-query", "synthetic-fragment"} {
		if bytes.Contains(output.Bytes(), []byte(marker)) {
			t.Fatal("report leaked an embedded URL credential")
		}
	}
}

func TestCLIAdministrationInitializationDoesNotAcquireCatalog(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	invoke := func(args ...string) (*App, []byte) {
		t.Helper()
		app := NewForCommand("test", "test", "test", "test")
		command := app.createRootCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(args)
		if err := command.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
		if app.runtime != nil || app.starmap != nil {
			t.Fatal("local administration initialized a catalog")
		}
		return app, output.Bytes()
	}
	app, output := invoke("admin", "init", "--id", "operator")
	var identity struct {
		Credential string `json:"credential"`
	}
	if err := json.Unmarshal(output, &identity); err != nil || identity.Credential == "" {
		t.Fatal("initialization returned no credential")
	}
	t.Setenv("STARMAP_ADMIN_TOKEN", identity.Credential)
	_, output = invoke("admin", "create", "--id", "gateway", "--role", "subscriber")
	var subscriber struct {
		Credential string `json:"credential"`
	}
	if err := json.Unmarshal(output, &subscriber); err != nil || subscriber.Credential == "" {
		t.Fatal("subscriber creation returned no credential")
	}
	cfg, err := app.AdministrationConfig()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := administration.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if principal, ok := manager.Authenticate(subscriber.Credential, cfg.Audience); !ok || principal.Role() != administration.Subscriber {
		t.Fatal("CLI created an invalid subscriber")
	}
}

func TestConfigurationReportPreservesPresenceStates(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv(catalogconfig.AcquisitionSources, "")
	t.Setenv(catalogconfig.SourceMaxAge, "0s")
	t.Setenv(catalogconfig.AcquisitionEnabled, "false")
	app := NewForCommand("test", "test", "test", "test")
	command := app.createRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"config", "effective"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Settings []reportedSetting `json:"settings"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	values := make(map[string]reportedSetting)
	for _, setting := range report.Settings {
		values[setting.Name] = setting
	}
	for _, test := range []struct {
		name, presence, value, origin string
		present                       bool
	}{
		{catalogconfig.AcquisitionSources, "empty", "", "environment", true},
		{catalogconfig.SourceMaxAge, "value", "0s", "environment", true},
		{catalogconfig.AcquisitionEnabled, "value", "false", "environment", true},
		{catalogconfig.ModelsDevGitCommit, "omitted", "", "default", false},
	} {
		got, exists := values[test.name]
		if !exists || got.Presence != test.presence || got.Value != test.value || got.Origin != test.origin || got.Present != test.present {
			t.Errorf("setting %s = %+v", test.name, got)
		}
	}
}

func TestInternalCatalogServeRequiresInitializedAdministration(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	t.Setenv(catalogconfig.Source, "starmap")
	t.Setenv(catalogconfig.SourceURL, "https://catalog.example.test")
	app := NewForCommand("test", "test", "test", "test")
	command := app.createRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"serve"})
	if err := command.ExecuteContext(t.Context()); err == nil || !strings.Contains(err.Error(), "starmap admin init") {
		t.Fatalf("internal startup = %v", err)
	}
	if app.runtime != nil || app.starmap != nil {
		t.Fatal("uninitialized internal server opened a catalog")
	}
}
