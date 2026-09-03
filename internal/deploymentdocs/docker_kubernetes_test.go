package deploymentdocs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

const (
	// dockerDocumentPath is the maintained document that owns the Kubernetes example.
	dockerDocumentPath = "../../docs/DOCKER.md"
	// kubernetesHeading starts the section that owns the example manifests.
	kubernetesHeading = "## Kubernetes"
	// starmapName is the Deployment and Service name of the catalog server.
	starmapName = "starmap"
	// starportName is the Deployment name of the gateway replica.
	starportName = "starport"
	// sourceURLName is the Starport setting that must name the Starmap Service.
	sourceURLName = "STARPORT_CATALOG_SOURCE_URL"
)

// header carries the kind and the name of one manifest.
type header struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

// deployment is the subset of a Deployment that the example must satisfy.
type deployment struct {
	Spec struct {
		Template struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec struct {
				Containers []struct {
					Name  string `yaml:"name"`
					Ports []struct {
						Name          string `yaml:"name"`
						ContainerPort int    `yaml:"containerPort"`
					} `yaml:"ports"`
					Env []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// service is the subset of a Service that the example must satisfy.
type service struct {
	Spec struct {
		Selector map[string]string `yaml:"selector"`
		Ports    []struct {
			Name       string `yaml:"name"`
			Port       int    `yaml:"port"`
			TargetPort string `yaml:"targetPort"`
		} `yaml:"ports"`
	} `yaml:"spec"`
}

// TestDockerDocumentKubernetesPairWiresStarportToStarmap parses the Kubernetes
// example and checks that it wires a Starport Deployment to a Starmap Service.
func TestDockerDocumentKubernetesPairWiresStarportToStarmap(t *testing.T) {
	t.Parallel()

	deployments := map[string]deployment{}
	services := map[string]service{}
	for _, block := range kubernetesBlocks(t) {
		var head header
		if err := yaml.Unmarshal([]byte(block), &head); err != nil {
			t.Fatalf("parse a Kubernetes block of %s: %v", dockerDocumentPath, err)
		}
		switch head.Kind {
		case "Deployment":
			var parsed deployment
			if err := yaml.Unmarshal([]byte(block), &parsed); err != nil {
				t.Fatalf("parse Deployment %q of %s: %v", head.Metadata.Name, dockerDocumentPath, err)
			}
			deployments[head.Metadata.Name] = parsed
		case "Service":
			var parsed service
			if err := yaml.Unmarshal([]byte(block), &parsed); err != nil {
				t.Fatalf("parse Service %q of %s: %v", head.Metadata.Name, dockerDocumentPath, err)
			}
			services[head.Metadata.Name] = parsed
		}
	}

	if len(deployments) != 2 {
		t.Fatalf("the Kubernetes example has %d named Deployment(s), want 2", len(deployments))
	}
	if len(services) != 1 {
		t.Fatalf("the Kubernetes example has %d named Service(s), want 1", len(services))
	}

	starmapDeployment, ok := deployments[starmapName]
	if !ok {
		t.Fatalf("the Kubernetes example has no Deployment named %q", starmapName)
	}
	starportDeployment, ok := deployments[starportName]
	if !ok {
		t.Fatalf("the Kubernetes example has no Deployment named %q", starportName)
	}
	starmapService, ok := services[starmapName]
	if !ok {
		t.Fatalf("the Kubernetes example has no Service named %q", starmapName)
	}

	assertSelectorMatchesStarmap(t, starmapService, starmapDeployment)
	servicePort := assertPortMatchesStarmap(t, starmapService, starmapDeployment)
	assertSourceURLNamesService(t, starportDeployment, servicePort)
}

// assertSelectorMatchesStarmap checks the Service selector against the Starmap pod labels.
func assertSelectorMatchesStarmap(t *testing.T, exposed service, starmap deployment) {
	t.Helper()

	if len(exposed.Spec.Selector) == 0 {
		t.Fatal("the Service declares no selector")
	}
	labels := starmap.Spec.Template.Metadata.Labels
	for key, want := range exposed.Spec.Selector {
		if labels[key] != want {
			t.Fatalf("the Service selector %s=%q does not match the Starmap pod label %q",
				key, want, labels[key])
		}
	}
}

// assertPortMatchesStarmap checks the Service port against the Starmap container
// port. It returns the exposed Service port.
func assertPortMatchesStarmap(t *testing.T, exposed service, starmap deployment) int {
	t.Helper()

	if len(exposed.Spec.Ports) != 1 {
		t.Fatalf("the Service declares %d port(s), want 1", len(exposed.Spec.Ports))
	}
	port := exposed.Spec.Ports[0]

	named := map[string]int{}
	for _, container := range starmap.Spec.Template.Spec.Containers {
		for _, containerPort := range container.Ports {
			named[containerPort.Name] = containerPort.ContainerPort
		}
	}
	containerPort, ok := named[port.TargetPort]
	if !ok {
		t.Fatalf("the Service target port %q names no Starmap container port", port.TargetPort)
	}
	if containerPort != port.Port {
		t.Fatalf("the Starmap container port %d does not match the Service port %d",
			containerPort, port.Port)
	}
	return port.Port
}

// assertSourceURLNamesService checks that the Starport source URL names the Service.
func assertSourceURLNamesService(t *testing.T, starport deployment, servicePort int) {
	t.Helper()

	sourceURL := ""
	for _, container := range starport.Spec.Template.Spec.Containers {
		for _, variable := range container.Env {
			if variable.Name == sourceURLName {
				sourceURL = variable.Value
			}
		}
	}
	if sourceURL == "" {
		t.Fatalf("the Starport Deployment sets no %s value", sourceURLName)
	}

	host := starmapName + ":" + strconv.Itoa(servicePort)
	if !strings.Contains(sourceURL, host) {
		t.Fatalf("%s is %q, which does not name the Service host %q",
			sourceURLName, sourceURL, host)
	}
}

// kubernetesBlocks returns the text of every YAML block under the Kubernetes heading.
func kubernetesBlocks(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(dockerDocumentPath))
	if err != nil {
		t.Fatalf("read %s: %v", dockerDocumentPath, err)
	}

	lines := strings.Split(string(data), "\n")
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == kubernetesHeading {
			start = index + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s has no %q heading", dockerDocumentPath, kubernetesHeading)
	}

	var blocks []string
	var block []string
	inBlock := false
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inBlock && strings.HasPrefix(trimmed, "## "):
			return checkedBlocks(t, blocks, inBlock)
		case !inBlock && trimmed == "```yaml":
			inBlock = true
			block = nil
		case inBlock && trimmed == "```":
			inBlock = false
			blocks = append(blocks, strings.Join(block, "\n"))
		case inBlock:
			block = append(block, line)
		}
	}
	return checkedBlocks(t, blocks, inBlock)
}

// checkedBlocks checks that the section closed every block and holds one manifest.
func checkedBlocks(t *testing.T, blocks []string, inBlock bool) []string {
	t.Helper()

	if inBlock {
		t.Fatalf("%s has an unterminated YAML block under %q", dockerDocumentPath, kubernetesHeading)
	}
	if len(blocks) == 0 {
		t.Fatalf("%s has no YAML block under %q", dockerDocumentPath, kubernetesHeading)
	}
	return blocks
}
