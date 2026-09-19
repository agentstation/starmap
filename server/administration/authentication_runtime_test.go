package administration

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthenticationRemainsInMemoryWithoutAllocations(t *testing.T) {
	manager, cfg, token, actor := initializedManager(t)
	if allocations := testing.AllocsPerRun(1000, func() {
		if _, ok := manager.Authenticate(token, cfg.Audience); !ok {
			t.Fatal("active credential rejected")
		}
	}); allocations != 0 {
		t.Fatalf("authentication allocations=%g, want zero", allocations)
	}
	state := filepath.Join(cfg.StateDirectory, "admin", identitiesFile)
	if err := os.Rename(state, state+".held"); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Authenticate(token, cfg.Audience); !ok {
		t.Fatal("authentication depends on a filesystem read")
	}
	if _, err := manager.Create(t.Context(), actor, "reader", Subscriber); err == nil {
		t.Fatal("a missing identity record accepted a mutation")
	}
}

func TestStreamingCredentialRevocationAndExpiryCancelRequests(t *testing.T) {
	for _, reason := range []string{"revoked", "expired", "closed"} {
		t.Run(reason, func(t *testing.T) {
			manager, cfg, _, actor := initializedManager(t)
			token, err := manager.Create(t.Context(), actor, "gateway", Subscriber)
			if err != nil {
				t.Fatal(err)
			}
			reader, ok := manager.Authenticate(token, cfg.Audience)
			if !ok {
				t.Fatal("reader authentication failed")
			}
			tracked, cancel := manager.Track(t.Context(), reader)
			defer cancel()
			switch reason {
			case "revoked":
				err = manager.Revoke(t.Context(), actor, "gateway")
			case "expired":
				_, err = manager.Rotate(t.Context(), actor, "gateway", 50*time.Millisecond)
			case "closed":
				err = manager.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-tracked.Done():
			case <-time.After(3 * time.Second):
				t.Fatal("stream survived withdrawn credential authority")
			}
		})
	}
}
