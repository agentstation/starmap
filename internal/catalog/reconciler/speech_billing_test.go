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

func TestImagePricingCopyIsolation(t *testing.T) {
	rate := 0.04
	source := &catalogs.ModelPricing{Operations: &catalogs.ModelOperationPricing{ImageUnit: &rate}}
	copied := copyModelPricing(source)
	*source.Operations.ImageUnit = 1
	if *copied.Operations.ImageUnit != 0.04 {
		t.Fatal("reconciled image unit shares source memory")
	}
}

func TestMediaDurationPricingCopyIsolation(t *testing.T) {
	input, output := 0.002, 0.075
	source := &catalogs.ModelPricing{Operations: &catalogs.ModelOperationPricing{InputSecond: &input, OutputSecond: &output}}
	copied := copyModelPricing(source)
	*source.Operations.InputSecond = 1
	*source.Operations.OutputSecond = 2
	if *copied.Operations.InputSecond != 0.002 || *copied.Operations.OutputSecond != 0.075 {
		t.Fatal("reconciled duration prices share source memory")
	}
}
