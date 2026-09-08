package settings_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/catalog/settings"
)

// composeService is the one service of the shipped Compose example.
type composeService struct {
	Image       string   `yaml:"image"`
	Command     []string `yaml:"command"`
	Environment []string `yaml:"environment"`
	Volumes     []string `yaml:"volumes"`
	User        string   `yaml:"user"`
	ReadOnly    bool     `yaml:"read_only"`
	Tmpfs       []string `yaml:"tmpfs"`
	CapDrop     []string `yaml:"cap_drop"`
	SecurityOpt []string `yaml:"security_opt"`
}

// composeFile is the shipped Compose example.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]struct {
		Driver string `yaml:"driver"`
	} `yaml:"volumes"`
}

// stateVolumePath is the container path that holds every writable file.
const stateVolumePath = "/home/nonroot"

// TestComposeExampleParsesAndPullsThePublicChannel proves the shipped Compose
// example. The running service sets no catalog setting, so it pulls the public
// channel. The commented block names every canonical setting. The container
// runs with a read-only root and one writable state volume.
func TestComposeExampleParsesAndPullsThePublicChannel(t *testing.T) {
	path := repositoryFile(t, "docker-compose.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var compose composeFile
	if err := yaml.Unmarshal(raw, &compose); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	service, held := compose.Services["starmap"]
	if !held {
		t.Fatal("the Compose file declares no starmap service")
	}

	// The running example sets no catalog setting, so the default source stays
	// the public channel.
	hasHome := false
	for _, entry := range service.Environment {
		name, value, _ := strings.Cut(entry, "=")
		if name == "STARMAP_HOME" {
			if value != stateVolumePath+"/starmap" {
				t.Fatalf("product home = %q, want the starmap child of the durable volume", value)
			}
			hasHome = true
			continue
		}
		if strings.HasPrefix(name, settings.Prefix) {
			t.Fatalf("the running service sets %q, so it does not pull the public channel", name)
		}
	}
	if !hasHome {
		t.Fatal("the service does not select its product home inside the durable volume")
	}

	// The service keeps a read-only root filesystem. One named volume holds
	// every writable file.
	if !service.ReadOnly {
		t.Fatal("the service does not run with a read-only root filesystem")
	}
	if len(service.Tmpfs) == 0 {
		t.Fatal("the service declares no writable temporary filesystem")
	}
	if service.User == "" {
		t.Fatal("the service names no unprivileged user")
	}
	assertHolds(t, service.CapDrop, "ALL", "cap_drop")
	assertHolds(t, service.SecurityOpt, "no-new-privileges:true", "security_opt")

	stateVolume := ""
	for _, volume := range service.Volumes {
		source, target, found := strings.Cut(volume, ":")
		if !found || target != stateVolumePath {
			continue
		}
		stateVolume = source
	}
	if stateVolume == "" {
		t.Fatalf("no volume mounts the writable state path %q", stateVolumePath)
	}
	if _, declared := compose.Volumes[stateVolume]; !declared {
		t.Fatalf("the Compose file declares no volume named %q", stateVolume)
	}
}

// TestDeploymentFilesDocumentEveryCanonicalName proves that the Compose example
// and the environment example name every canonical catalog setting.
// It rejects unknown names while retaining recognized node-owned settings.
func TestDeploymentFilesDocumentEveryCanonicalName(t *testing.T) {
	for _, name := range []string{"docker-compose.yml", ".env.example"} {
		path := repositoryFile(t, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(raw)
		for _, setting := range settings.Names() {
			if !strings.Contains(content, setting) {
				t.Fatalf("%s names no %s", name, setting)
			}
		}
		assertNoUnknownCatalogName(t, name, content)
	}
}

// assertNoUnknownCatalogName rejects unknown catalog and node setting names.
func assertNoUnknownCatalogName(t *testing.T, file, content string) {
	t.Helper()
	for _, found := range unknownDeploymentNames(content) {
		t.Fatalf("%s names the unknown setting %s", file, found)
	}
}

func unknownDeploymentNames(content string) []string {
	known := make(map[string]bool, len(settings.Names()))
	for _, name := range settings.Names() {
		known[name] = true
	}
	// The application owns path anchor intent separately from catalog settings.
	known["STARMAP_RELATIVE_PATH_BASE"] = true
	known["STARMAP_HOME"] = true
	var unknown []string
	pattern := regexp.MustCompile(`STARMAP_[A-Z0-9_]+`)
	for _, found := range pattern.FindAllString(content, -1) {
		if !known[found] {
			unknown = append(unknown, found)
		}
	}
	return unknown
}

func TestDeploymentNameValidationPreservesUnknownNameRefusal(t *testing.T) {
	if names := unknownDeploymentNames("STARMAP_HOME STARMAP_RELATIVE_PATH_BASE " + settings.Source); len(names) != 0 {
		t.Fatalf("recognized node or catalog name rejected: %v", names)
	}
	for _, typo := range []string{"STARMAP_HOME_TYPO", "STARMAP_RELATIVE_PATH_BASE_TYPO", "STARMAP_CATALOG_SOURCE_TYPO"} {
		if names := unknownDeploymentNames(typo); len(names) != 1 || names[0] != typo {
			t.Fatalf("unknown name accepted: %q", typo)
		}
	}
}

// assertHolds proves that a list carries one required value.
func assertHolds(t *testing.T, values []string, want, field string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%s = %v, want it to hold %q", field, values, want)
}

// repositoryFile returns the absolute path of one repository file.
func repositoryFile(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", name))
	if err != nil {
		t.Fatalf("resolve %s: %v", name, err)
	}
	return path
}
