package deploymentdocs

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type containerMount struct {
	Path     string `yaml:"mountPath"`
	ReadOnly bool   `yaml:"readOnly"`
}

func TestComposeDeclaresMountedApplicationRoots(t *testing.T) {
	data, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	var recipe struct {
		Services map[string]struct {
			Environment []string `yaml:"environment"`
			Volumes     []string `yaml:"volumes"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &recipe); err != nil {
		t.Fatal(err)
	}
	service, ok := recipe.Services[starmapName]
	if !ok {
		t.Fatal("Compose has no Starmap service")
	}
	values := make(map[string]string)
	for _, entry := range service.Environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	var mounts []containerMount
	for _, volume := range service.Volumes {
		parts := strings.Split(volume, ":")
		if len(parts) >= 2 {
			mounts = append(mounts, containerMount{Path: parts[1], ReadOnly: len(parts) == 3 && parts[2] == "ro"})
		}
	}
	assertMountedRoots(t, values, mounts)
}

func TestKubernetesDeclaresMountedApplicationRoots(t *testing.T) {
	for _, block := range kubernetesBlocks(t) {
		var h header
		if err := yaml.Unmarshal([]byte(block), &h); err != nil {
			t.Fatal(err)
		}
		if h.Kind != "Deployment" || h.Metadata.Name != starmapName {
			continue
		}
		var recipe struct {
			Spec struct {
				Replicas int `yaml:"replicas"`
				Strategy struct {
					Type string `yaml:"type"`
				} `yaml:"strategy"`
				Template struct {
					Spec struct {
						Containers []struct {
							Name string `yaml:"name"`
							Env  []struct {
								Name, Value string
							} `yaml:"env"`
							Mounts []containerMount `yaml:"volumeMounts"`
						} `yaml:"containers"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal([]byte(block), &recipe); err != nil {
			t.Fatal(err)
		}
		if recipe.Spec.Replicas != 1 || recipe.Spec.Strategy.Type != "Recreate" {
			t.Fatal("filesystem deployment permits overlapping writers during a rollout")
		}
		for _, container := range recipe.Spec.Template.Spec.Containers {
			if container.Name != starmapName {
				continue
			}
			values := make(map[string]string)
			for _, env := range container.Env {
				values[env.Name] = env.Value
			}
			assertMountedRoots(t, values, container.Mounts)
			return
		}
	}
	t.Fatal("Kubernetes example has no Starmap container")
}

func assertMountedRoots(t *testing.T, values map[string]string, mounts []containerMount) {
	t.Helper()
	for role, selector := range map[string]string{"config": "STARMAP_CONFIG_DIR", "data": "STARMAP_DATA_DIR", "state": "STARMAP_STATE_ROOT", "cache": "STARMAP_CACHE_DIR"} {
		selected, explicit := values[selector]
		if !explicit && values["STARMAP_HOME"] != "" {
			selected = path.Join(values["STARMAP_HOME"], role)
		}
		if !path.IsAbs(selected) {
			t.Errorf("%s has no explicit absolute container root", role)
			continue
		}
		covered := false
		for _, mount := range mounts {
			if role != "config" && mount.ReadOnly {
				continue
			}
			if selected == mount.Path || strings.HasPrefix(selected, path.Clean(mount.Path)+"/") {
				covered = true
			}
		}
		if !covered {
			t.Errorf("%s root %q has no suitable mount", role, selected)
		}
	}
}
