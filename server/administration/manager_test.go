package administration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func initializedManager(t *testing.T) (*Manager, Config, string, Principal) {
	t.Helper()
	cfg := Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "enterprise/catalog"}
	manager, token, err := Initialize(t.Context(), cfg, "operator")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	principal, accepted := manager.Authenticate(token, cfg.Audience)
	if !accepted || principal.Role() != Administrator {
		t.Fatal("bootstrap credential was not accepted")
	}
	return manager, cfg, token, principal
}

func TestFirstAdministratorBootstrapAndRestore(t *testing.T) {
	manager, cfg, token, actor := initializedManager(t)
	if _, _, err := Initialize(t.Context(), cfg, "other"); err == nil {
		t.Fatal("second process acquired initialized state")
	}
	readerToken, err := manager.Create(t.Context(), actor, "gateway", Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	reader, accepted := manager.Authenticate(readerToken, cfg.Audience)
	if !accepted || reader.Role() != Subscriber {
		t.Fatal("subscriber credential was not accepted")
	}
	if _, err := manager.StartOperation(t.Context(), reader, RefreshCatalog, "catalog"); err == nil {
		t.Fatal("subscriber authorized a mutation")
	}
	if _, err := manager.Create(t.Context(), Principal{}, "forged", Administrator); err == nil {
		t.Fatal("zero principal authorized a mutation")
	}
	if _, accepted := manager.Authenticate(token, "another/catalog"); accepted {
		t.Fatal("credential crossed the audience boundary")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if _, accepted := manager.Authenticate(token, cfg.Audience); accepted {
		t.Fatal("closed manager authenticated a caller")
	}
	restored, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if !restored.Status().MutationReady || restored.Status().Revision != 2 {
		t.Fatalf("restore status: %+v", restored.Status())
	}
	for _, credential := range []string{token, readerToken} {
		if _, accepted := restored.Authenticate(credential, cfg.Audience); !accepted {
			t.Fatal("restore lost a credential")
		}
	}
	if _, _, err := Initialize(t.Context(), cfg, "other"); err == nil {
		t.Fatal("initialization overwrote existing state")
	}
	if err := filepath.WalkDir(cfg.StateDirectory, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if bytes.Contains(data, []byte(token)) || bytes.Contains(data, []byte(readerToken)) {
			t.Error("credential persisted without hashing")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDurableCredentialRotationAndRevocation(t *testing.T) {
	manager, cfg, _, actor := initializedManager(t)
	token, err := manager.Create(t.Context(), actor, "gateway", Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(time.Minute)
	manager.clock = func() time.Time { return now }
	next, err := manager.Rotate(t.Context(), actor, "gateway", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, credential := range []string{token, next} {
		if _, accepted := manager.Authenticate(credential, cfg.Audience); !accepted {
			t.Fatal("rotation overlap rejected a valid credential")
		}
	}
	if _, err := manager.Rotate(t.Context(), actor, "gateway", time.Minute); err == nil {
		t.Fatal("rotation discarded an unexpired overlap")
	}
	now = now.Add(time.Minute)
	if _, accepted := manager.Authenticate(token, cfg.Audience); accepted {
		t.Fatal("old credential survived its overlap")
	}
	if _, accepted := manager.Authenticate(next, cfg.Audience); !accepted {
		t.Fatal("new credential expired with the old credential")
	}
	if err := manager.Revoke(t.Context(), actor, "gateway"); err != nil {
		t.Fatal(err)
	}
	if _, accepted := manager.Authenticate(next, cfg.Audience); accepted {
		t.Fatal("revoked credential remains accepted")
	}
	if err := manager.Revoke(t.Context(), actor, actor.ID()); err == nil {
		t.Fatal("last administrator was revoked")
	}
}

func TestRevokedPrincipalCannotAuthorizeLaterMutation(t *testing.T) {
	manager, cfg, _, first := initializedManager(t)
	secondToken, err := manager.Create(t.Context(), first, "second", Administrator)
	if err != nil {
		t.Fatal(err)
	}
	second, ok := manager.Authenticate(secondToken, cfg.Audience)
	if !ok {
		t.Fatal("second administrator was not accepted")
	}
	if err := manager.Revoke(t.Context(), second, first.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Create(t.Context(), first, "forged", Administrator); err == nil {
		t.Fatal("stale principal authorized a mutation")
	}
}

func TestAuditFailureRejectsMutationAndPreservesDiagnostics(t *testing.T) {
	manager, cfg, token, actor := initializedManager(t)
	before, err := os.ReadFile(filepath.Join(cfg.StateDirectory, "admin", identitiesFile))
	if err != nil {
		t.Fatal(err)
	}
	manager.audit.beforeAppend = func() error { return syscall.ENOSPC }
	if _, err := manager.Create(t.Context(), actor, "unrecorded", Subscriber); err == nil {
		t.Fatal("disk-full mutation succeeded")
	}
	after, err := os.ReadFile(filepath.Join(cfg.StateDirectory, "admin", identitiesFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("mutation changed identities before its audit intent")
	}
	if _, accepted := manager.Authenticate(token, cfg.Audience); !accepted {
		t.Fatal("audit failure disabled diagnostic authentication")
	}
	if manager.Status().MutationReady {
		t.Fatal("audit failure left mutations enabled")
	}
}

func TestAuditOutcomeFailureRetainsRevocationAndRecovers(t *testing.T) {
	manager, cfg, adminToken, actor := initializedManager(t)
	readerToken, err := manager.Create(t.Context(), actor, "gateway", Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	manager.audit.beforeAppend = func() error {
		writes++
		if writes == 2 {
			return syscall.ENOSPC
		}
		return nil
	}
	if err := manager.Revoke(t.Context(), actor, "gateway"); err == nil {
		t.Fatal("unrecorded outcome reported success")
	}
	if _, accepted := manager.Authenticate(readerToken, cfg.Audience); accepted {
		t.Fatal("audit failure undid a published revocation")
	}
	if _, accepted := manager.Authenticate(adminToken, cfg.Audience); !accepted {
		t.Fatal("administrator diagnostics unavailable")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if !restored.Status().MutationReady {
		t.Fatal("proved identity outcome was not recovered")
	}
	if _, accepted := restored.Authenticate(readerToken, cfg.Audience); accepted {
		t.Fatal("restart restored a revoked key")
	}
	for _, receipt := range restored.audit.receipts {
		if receipt.Action == RevokeIdentity && receipt.State != Succeeded {
			t.Fatal("recovery did not prove revocation")
		}
	}
}

func TestReceiptRecoveryNeverReplaysUnknownEffects(t *testing.T) {
	manager, cfg, token, actor := initializedManager(t)
	receipt, err := manager.StartOperation(t.Context(), actor, RefreshCatalog, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	actor, accepted := restored.Authenticate(token, cfg.Audience)
	if !accepted {
		t.Fatal("restored operator unavailable")
	}
	recovered, err := restored.Operation(actor, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State != Interrupted {
		t.Fatalf("unknown effect became %s", recovered.State)
	}
	if _, err := restored.Complete(t.Context(), receipt.ID, true); err == nil {
		t.Fatal("unknown effect was retroactively marked successful")
	}
	sequence := restored.audit.sequence
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if again.audit.sequence != sequence {
		t.Fatal("repeated recovery duplicated an audit outcome")
	}
}

func TestMalformedAuditAllowsOnlyAuthenticatedDiagnostics(t *testing.T) {
	manager, cfg, token, _ := initializedManager(t)
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(cfg.StateDirectory, "admin", "audit", auditFile), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"interrupted":`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	actor, accepted := restored.Authenticate(token, cfg.Audience)
	if !accepted {
		t.Fatal("diagnostics lost authentication during audit corruption")
	}
	if restored.Status().MutationReady {
		t.Fatal("corrupt audit permitted mutations")
	}
	if _, err := restored.Create(t.Context(), actor, "blocked", Subscriber); err == nil {
		t.Fatal("corrupt audit accepted mutation")
	}
}

func TestConcurrentAdministrativeMutationsHaveOrderedDurableReceipts(t *testing.T) {
	manager, _, _, actor := initializedManager(t)
	var group sync.WaitGroup
	for i := range 8 {
		group.Go(func() {
			id := "reader-" + strings.Repeat("x", i+1)
			if _, err := manager.Create(context.Background(), actor, id, Subscriber); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if manager.Status().Revision != 9 || manager.audit.sequence != 18 {
		t.Fatalf("status %+v, sequence %d", manager.Status(), manager.audit.sequence)
	}
}

func TestLocalAdministratorRecoveryPreservesAuditAndRevokesLostKey(t *testing.T) {
	manager, cfg, old, _ := initializedManager(t)
	if _, err := RecoverAdministrator(t.Context(), cfg, "operator"); err == nil {
		t.Fatal("local recovery bypassed the live writer")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
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
	if _, ok := restored.Authenticate(old, cfg.Audience); ok {
		t.Fatal("lost credential survived local recovery")
	}
	if _, ok := restored.Authenticate(token, cfg.Audience); !ok {
		t.Fatal("recovered administrator credential failed")
	}
	if restored.record.Revision != 2 || restored.audit.sequence != 4 {
		t.Fatal("local recovery reset identity or audit history")
	}
}

func TestMissingAuditDirectoryPreservesAuthenticatedDiagnostics(t *testing.T) {
	manager, cfg, token, _ := initializedManager(t)
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(cfg.StateDirectory, "admin", "audit")
	if err := os.Rename(directory, directory+".held"); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, ok := restored.Authenticate(token, cfg.Audience); !ok {
		t.Fatal("unavailable audit state disabled diagnostic authentication")
	}
	if restored.Status().MutationReady {
		t.Fatal("missing audit directory permits mutations")
	}
}

func TestAuditCapacityRefusesMutationAndReportsLimits(t *testing.T) {
	manager, _, token, actor := initializedManager(t)
	manager.audit.entry.Size = maximumAuditBytes
	status := manager.Status()
	if status.MutationReady || status.Reason != "retained_history_capacity" || status.AuditByteLimit != maximumAuditBytes || status.OperationLimit != maximumOperations {
		t.Fatalf("capacity status=%+v", status)
	}
	if _, err := manager.Create(t.Context(), actor, "blocked", Subscriber); err == nil {
		t.Fatal("capacity accepted another mutation")
	}
	if _, ok := manager.Authenticate(token, manager.Audience()); !ok {
		t.Fatal("capacity removed diagnostic authentication")
	}
}
