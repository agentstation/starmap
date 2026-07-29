package consumer

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/server"
)

func TestVerifyRemoteCatalog(t *testing.T) {
	constructionCtx, constructionCancel := context.WithTimeout(
		context.Background(),
		time.Minute,
	)
	defer constructionCancel()
	client, err := starmap.NewContext(constructionCtx)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	config := server.DefaultConfig()
	config.RateLimit = 0
	config.MetricsEnabled = false
	starmapServer, err := server.New(client, config)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := starmapServer.Start(); err != nil {
		t.Fatalf("server.Start: %v", err)
	}
	httpServer := httptest.NewServer(starmapServer.Handler())
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer shutdownCancel()
		if err := starmapServer.Shutdown(shutdownCtx); err != nil {
			t.Errorf("server.Shutdown: %v", err)
		}
	}()
	defer httpServer.Close()

	subscriberCtx, subscriberCancel := context.WithTimeout(
		context.Background(),
		time.Minute,
	)
	defer subscriberCancel()
	if err := VerifyRemoteCatalog(
		subscriberCtx,
		httpServer.URL+"/api/v1",
	); err != nil {
		t.Fatalf("VerifyRemoteCatalog: %v", err)
	}
}
