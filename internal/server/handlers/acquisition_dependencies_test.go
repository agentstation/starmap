package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcquisitionDependencyDetailSanitizesTypedFailures(t *testing.T) {
	const secret = "private-error-command-token"
	for _, test := range []struct {
		name, source, dependency   string
		wantSource, wantDependency bool
	}{
		{"known", "models_dev_git", "git", true, true},
		{"bun", "models_dev_git", "bun", true, true},
		{"unknown source", secret, "git", false, false},
		{"unknown tool", "models_dev_git", secret, true, false},
		{"absent source", "", "git", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := &pkgerrors.ConfigError{Component: secret, Message: secret, Err: &pkgerrors.DependencyError{Source: test.source, Dependency: test.dependency, Message: secret}}
			detail := acquisitionDependencyDetail(err)
			if (detail != nil) != test.wantSource {
				t.Fatalf("detail=%v", detail)
			}
			if _, present := detail["dependency"]; present != test.wantDependency {
				t.Fatalf("dependency detail=%v", detail)
			}
			if detail != nil && (detail["source"] != test.source || detail["action"] != "check_source_dependencies") {
				t.Fatalf("source detail=%v", detail)
			}
			raw, marshalErr := json.Marshal(detail)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if strings.Contains(string(raw), secret) {
				t.Fatal("source detail exposed untrusted error text")
			}
			reason := sources.ClassifyProviderReason(err)
			if reason != sources.ProviderReasonDependencyUnavailable || !reason.Valid() {
				t.Fatalf("dependency reason=%s", reason)
			}
		})
	}
	if acquisitionDependencyDetail(&pkgerrors.ConfigError{Message: secret}) != nil {
		t.Fatal("untyped source failure produced dependency advice")
	}
}
