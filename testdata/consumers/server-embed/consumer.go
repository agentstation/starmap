// Package consumer is a real external module exercising the public embeddable
// Starmap server without importing CLI or internal packages.
package consumer

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/server"
)

// ServeAndShutdown proves the public server can be constructed, served,
// reached, gracefully drained, and stopped by an external Go program.
func ServeAndShutdown(ctx context.Context) error {
	client, err := starmap.NewContext(ctx)
	if err != nil {
		return err
	}
	config := server.DefaultConfig()
	config.RateLimit = 0
	config.MetricsEnabled = false
	srv, err := server.New(client, config)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	stopped := false
	defer func() {
		if stopped {
			return
		}
		_ = listener.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(cleanupCtx)
	}()
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- srv.Serve(listener)
	}()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://"+listener.Addr().String()+"/health",
		nil,
	)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	updateRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://"+listener.Addr().String()+"/api/v1/update",
		nil,
	)
	if err != nil {
		return err
	}
	updateResponse, err := http.DefaultClient.Do(updateRequest)
	if err != nil {
		return err
	}
	_, copyErr = io.Copy(io.Discard, updateResponse.Body)
	closeErr = updateResponse.Body.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if updateResponse.StatusCode != http.StatusNotFound {
		return fmt.Errorf(
			"unconfigured update status = %d, want %d",
			updateResponse.StatusCode,
			http.StatusNotFound,
		)
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	stopped = true
	select {
	case err := <-serveResult:
		return err
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}
}
