package administration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInterruptedFirstAdministratorRecovery(t *testing.T) {
	cfg := Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}
	writer, err := openStateWriter(t.Context(), cfg.StateDirectory, true)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := openAudit(t.Context(), writer.audit, cfg.Audience, true)
	if err != nil {
		t.Fatal(err)
	}
	manager := &Manager{writer: writer, audit: audit, audience: cfg.Audience, clock: time.Now, changed: make(chan struct{})}
	_, key, err := newCredential(Administrator, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	record := identitiesRecord{SchemaVersion: identitySchemaVersion, Audience: cfg.Audience, Revision: 1,
		Identities: []identityRecord{{ID: "operator", Role: Administrator, Keys: []credentialRecord{key}}}}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := manager.begin(t.Context(), "operator", Bootstrap, "operator", "", contentDigest(data), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Initialize(t.Context(), cfg, "operator"); err == nil {
		t.Fatal("initialization silently replaced interrupted history")
	}
	if _, err := RecoverAdministrator(t.Context(), cfg, "different"); err == nil {
		t.Fatal("recovery ignored the original administrator identity")
	}
	token, err := RecoverAdministrator(t.Context(), cfg, "operator")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	actor, ok := restored.Authenticate(token, cfg.Audience)
	if !ok || actor.Role() != Administrator || !restored.Status().MutationReady {
		t.Fatal("recovery did not restore administration")
	}
	previous, err := restored.Operation(actor, receipt.ID)
	if err != nil || previous.State != Interrupted {
		t.Fatalf("interrupted bootstrap was not retained: %v, %v", previous.State, err)
	}
	if len(restored.audit.receipts) != 2 {
		t.Fatal("recovery replaced audit history")
	}
}

func TestRecoveryDoesNotReplaceDeletedInitializedIdentities(t *testing.T) {
	cfg := Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}
	manager, _, err := Initialize(t.Context(), cfg, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(cfg.StateDirectory, "admin", identitiesFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverAdministrator(t.Context(), cfg, "operator"); err == nil {
		t.Fatal("recovery ignored missing identities after successful initialization")
	}
}
