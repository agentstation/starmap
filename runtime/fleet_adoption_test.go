package runtime

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"
)

func TestFleetSnapshotAdoptionPreservesPublication(t *testing.T) {
	publication := fleetTestPublication(t)
	original := publication.nextHead()
	recovered := original
	recovered.Identity.RecoveryEpoch++
	recovered.Identity.BackendID = "replacement"
	wire, err := json.Marshal(map[string]any{
		"head": recovered, "publication": publication,
		"adoption": map[string]any{"previous": original, "receipt": strings.Repeat("a", 64)},
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot FleetSnapshot
	if err := json.Unmarshal(wire, &snapshot); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(publication, snapshot.Publication) {
		t.Fatal("recovery changed the original publication")
	}
}

func TestFleetSnapshotAdoptionRejectsChangedEvidence(t *testing.T) {
	publication := fleetTestPublication(t)
	previous := publication.nextHead()
	head := previous
	head.Identity.RecoveryEpoch++
	head.Identity.BackendID = "replacement"
	for _, scenario := range []struct {
		name   string
		change func(*FleetSnapshot)
	}{
		{"receipt", func(s *FleetSnapshot) { s.Adoption.Receipt = "invalid" }},
		{"previous-empty", func(s *FleetSnapshot) { s.Adoption.Previous = FleetHead{} }},
		{"same-epoch", func(s *FleetSnapshot) { s.Head.Identity.RecoveryEpoch = previous.Identity.RecoveryEpoch }},
		{"previous-regression", func(s *FleetSnapshot) { s.Adoption.Previous.Identity.RecoveryEpoch-- }},
		{"parallel-backend", func(s *FleetSnapshot) { s.Adoption.Previous.Identity.BackendID = "different" }},
		{"foreign-deployment", func(s *FleetSnapshot) { s.Head.Identity.DeploymentID = "other" }},
		{"foreign-previous", func(s *FleetSnapshot) { s.Adoption.Previous.Identity.DeploymentID = "other" }},
		{"revision", func(s *FleetSnapshot) { s.Head.Revision++ }},
		{"generation", func(s *FleetSnapshot) { s.Head.GenerationID = "different" }},
		{"recovery-inputs", func(s *FleetSnapshot) { s.Head.RecoveryChecksum = strings.Repeat("b", 64) }},
		{"previous-revision", func(s *FleetSnapshot) { s.Adoption.Previous.Revision++ }},
		{"original-grant", func(s *FleetSnapshot) { s.Publication.Grant.Identity.BackendID = "different" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			snapshot := FleetSnapshot{Head: head, Publication: publication, Adoption: &FleetAdoption{Previous: previous, Receipt: strings.Repeat("a", 64)}}
			scenario.change(&snapshot)
			if snapshot.Validate() == nil {
				t.Fatal("accepted inconsistent adoption evidence")
			}
		})
	}
	snapshot := FleetSnapshot{Head: head, Publication: publication, Adoption: &FleetAdoption{Previous: previous, Receipt: strings.Repeat("a", 64)}}
	snapshot.Adoption.Previous = head
	snapshot.Head.Identity.RecoveryEpoch++
	snapshot.Head.Identity.BackendID = "third-backend"
	if err := snapshot.Validate(); err != nil {
		t.Fatal("refused repeated explicit recovery:", err)
	}
}
