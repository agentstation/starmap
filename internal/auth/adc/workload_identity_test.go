package adc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildDetailsRecognizesWorkloadIdentity(t *testing.T) {
	credentialPath := filepath.Join(t.TempDir(), "workload-identity.json")
	credential := []byte(`{
  "type": "external_account",
  "audience": "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
  "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
  "token_url": "https://sts.googleapis.com/v1/token",
  "credential_source": {"file": "/var/run/secrets/workload-token"}
}`)
	if err := os.WriteFile(credentialPath, credential, 0o600); err != nil {
		t.Fatalf("write credential fixture: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentialPath)
	t.Setenv("GOOGLE_VERTEX_PROJECT", "workload-project")

	details := BuildDetails()
	if details.State != StateConfigured || details.Type != "Workload Identity" {
		t.Fatalf("details = %#v", details)
	}
	if details.Project != "workload-project" {
		t.Fatalf("project = %q, want workload-project", details.Project)
	}
}
