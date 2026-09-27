package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestSpeechPricingCopyIsolation(t *testing.T) {
	rate := 0.000015
	source := &catalogs.ModelPricing{Operations: &catalogs.ModelOperationPricing{CharacterInput: &rate}}
	copied := copyModelPricing(source)
	*source.Operations.CharacterInput = 1
	if *copied.Operations.CharacterInput != 0.000015 {
		t.Fatal("reconciled character price shares source memory")
	}
}
