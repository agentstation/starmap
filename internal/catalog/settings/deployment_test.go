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
	for _, entry := range service.Environment {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, settings.Prefix) {
			t.Fatalf("the running service sets %q, so it does not pull the public channel", name)
		}
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
// and the environment example name every canonical catalog setting. It also
// proves that neither file names any other catalog setting, so a removed name
// fails the test.
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

// assertNoUnknownCatalogName proves that a deployment file invents no catalog
// setting name. Only the canonical table names a setting.
func assertNoUnknownCatalogName(t *testing.T, file, content string) {
	t.Helper()
	known := make(map[string]bool, len(settings.Names()))
	for _, name := range settings.Names() {
		known[name] = true
	}
	pattern := regexp.MustCompile(`STARMAP_[A-Z0-9_]+`)
	for _, found := range pattern.FindAllString(content, -1) {
		if !known[found] {
			t.Fatalf("%s names the unknown catalog setting %s", file, found)
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
