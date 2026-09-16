package administration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAdministrationProcessCrashRecovery(t *testing.T) {
	if phase := os.Getenv("STARMAP_TEST_ADMIN_CRASH"); phase != "" {
		cfg := Config{StateDirectory: os.Getenv("STARMAP_TEST_ADMIN_STATE"), Audience: "test"}
		manager, token, err := Initialize(t.Context(), cfg, "operator")
		if err != nil {
			t.Fatal(err)
		}
		actor, _ := manager.Authenticate(token, cfg.Audience)
		if phase == "after-intent" {
			if _, err := manager.StartOperation(t.Context(), actor, RefreshCatalog, "catalog"); err != nil {
				t.Fatal(err)
			}
			os.Exit(23)
		}
		writes := 0
		manager.audit.beforeAppend = func() error {
			writes++
			if writes == 2 {
				os.Exit(23)
			}
			return nil
		}
		if _, err := manager.Create(t.Context(), actor, "gateway", Subscriber); err != nil {
			t.Fatal(err)
		}
		t.Fatal("crash injection was not reached")
	}
	for _, phase := range []string{"after-intent", "after-identity"} {
		t.Run(phase, func(t *testing.T) {
			cfg := Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), executable, "-test.run=^TestAdministrationProcessCrashRecovery$")
			command.Env = append(os.Environ(), "STARMAP_TEST_ADMIN_CRASH="+phase, "STARMAP_TEST_ADMIN_STATE="+cfg.StateDirectory)
			output, err := command.CombinedOutput()
			if failure, ok := err.(*exec.ExitError); !ok || failure.ExitCode() != 23 {
				t.Fatalf("crash child: %v\n%s", err, output)
			}
			restored, err := Open(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer restored.Close()
			if !restored.Status().MutationReady {
				t.Fatal("recovery did not restore administration readiness")
			}
			found := false
			for _, receipt := range restored.audit.receipts {
				if receipt.Action == Bootstrap {
					continue
				}
				found = true
				want := Interrupted
				if phase == "after-identity" {
					want = Succeeded
				}
				if receipt.State != want {
					t.Fatalf("recovery state=%s want=%s", receipt.State, want)
				}
				if err := restored.checkReceipt(receipt); err != nil {
					t.Fatal(err)
				}
			}
			if !found {
				t.Fatal("recovery lost the interrupted operation")
			}
		})
	}
}
