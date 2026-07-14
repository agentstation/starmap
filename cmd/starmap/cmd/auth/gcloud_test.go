package auth

import (
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestVertexProjectEnvironmentNamesComeFromProviderConfiguration(t *testing.T) {
	got := vertexProjectEnvironmentNames()
	want := catalogs.ProviderEnvironmentNames{"GOOGLE_CLOUD_PROJECT", "GOOGLE_VERTEX_PROJECT"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("project environment names = %#v, want %#v", got, want)
	}
}
