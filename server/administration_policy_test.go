package server

import (
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/runtime/status"
	"github.com/agentstation/starmap/server/administration"
)

type administrationRequiredRuntime struct{ report status.Status }

func (r administrationRequiredRuntime) Status() status.Status { return r.report }
func (administrationRequiredRuntime) Close() error            { return nil }

func TestInternalServerEmbeddingRequiresManagedReaders(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	manager, _, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "enterprise"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	for _, report := range []status.Status{{AuthorityRequired: true}, {SourceKind: status.SourceStarmap}} {
		connected := administrationRequiredRuntime{report: report}
		if _, err := New(client, DefaultConfig(), WithRuntime(connected)); err == nil {
			t.Fatal("internal server accepted no managed identities")
		}
		if _, err := New(client, DefaultConfig(), WithRuntime(connected), WithAdministration(manager, "enterprise")); err != nil {
			t.Fatal(err)
		}
	}
}
