package catalogdistribution

import (
	"os"
	"strings"
	"testing"
)

func TestArtifactDistributionTrustTradeoffsDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/CATALOG_DISTRIBUTION_TRUST.md")
	if err != nil {
		t.Fatalf("Read trust model: %v", err)
	}
	document := string(data)
	for _, required := range []string{
		"Embedded bootstrap", "GitHub Release assets", "Hosted `starmap.agentstation.ai`", "OCI mirror",
		"Trust root", "Freshness and availability", "Principal risks", "Intended policy",
		"last-known-good", "repository/workflow", "Air-gapped", "Restricted-egress",
	} {
		if !strings.Contains(document, required) {
			t.Errorf("trust model is missing %q", required)
		}
	}
}
