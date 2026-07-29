// Package consumer is a real external module exercising the public embeddable
// Starmap server without importing CLI or internal packages.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
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
	health := srv.Health()
	if health.State != server.StateServing || health.ActiveGenerationID == "" ||
		health.CatalogGeneratedAt.IsZero() {
		return fmt.Errorf("unexpected serving health: %#v", health)
	}
	if err := validateOpenRouterCatalog(
		ctx,
		listener.Addr().String(),
		client.Catalog(),
	); err != nil {
		return err
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
	if health := srv.Health(); health.State != server.StateStopped ||
		health.Stream.State != server.StreamStateStopped {
		return fmt.Errorf("unexpected stopped health: %#v", health)
	}
	stopped = true
	select {
	case err := <-serveResult:
		return err
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}
}

func validateOpenRouterCatalog(
	ctx context.Context,
	address string,
	catalog *catalogs.Catalog,
) error {
	author, slug, err := selectOpenRouterModel(catalog)
	if err != nil {
		return err
	}
	modelID := author + "/" + slug
	escapedAuthor := url.PathEscape(author)
	escapedSlug := url.PathEscape(slug)
	for _, route := range []struct {
		path string
		read func(*json.Decoder) error
	}{
		{
			path: "/api/v1/model/" + escapedAuthor + "/" + escapedSlug,
			read: func(decoder *json.Decoder) error {
				var response struct {
					Data struct {
						ID            string `json:"id"`
						CanonicalSlug string `json:"canonical_slug"`
					} `json:"data"`
				}
				if err := decoder.Decode(&response); err != nil {
					return err
				}
				if response.Data.ID != modelID ||
					response.Data.CanonicalSlug != modelID {
					return fmt.Errorf("unexpected OpenRouter model: %#v", response.Data)
				}
				return nil
			},
		},
		{
			path: "/api/v1/models/" + escapedAuthor + "/" + escapedSlug + "/endpoints",
			read: func(decoder *json.Decoder) error {
				var response struct {
					Data struct {
						ID        string `json:"id"`
						Endpoints []struct {
							ModelID string `json:"model_id"`
							Tag     string `json:"tag"`
						} `json:"endpoints"`
					} `json:"data"`
				}
				if err := decoder.Decode(&response); err != nil {
					return err
				}
				if response.Data.ID != modelID ||
					len(response.Data.Endpoints) == 0 ||
					response.Data.Endpoints[0].ModelID == "" ||
					response.Data.Endpoints[0].Tag == "" {
					return fmt.Errorf("unexpected OpenRouter endpoints: %#v", response.Data)
				}
				return nil
			},
		},
	} {
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://"+address+route.path,
			nil,
		)
		if err != nil {
			return err
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return fmt.Errorf(
				"OpenRouter route %s status = %d, want %d",
				route.path,
				response.StatusCode,
				http.StatusOK,
			)
		}
		readErr := route.read(json.NewDecoder(response.Body))
		closeErr := response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func selectOpenRouterModel(catalog *catalogs.Catalog) (string, string, error) {
	if catalog == nil {
		return "", "", fmt.Errorf("catalog is nil")
	}
	for _, definition := range catalog.Definitions() {
		offerings, err := catalog.DefinitionOfferings(definition.ID)
		if err != nil {
			return "", "", err
		}
		for _, offering := range offerings {
			if offering.Availability == catalogs.OfferingAvailabilityUnavailable ||
				offering.Lifecycle == catalogs.OfferingLifecycleRetired {
				continue
			}
			author, slug, found := strings.Cut(string(definition.ID), "/")
			if !found || author == "" || slug == "" {
				return "", "", fmt.Errorf(
					"unexpected canonical model ID %q",
					definition.ID,
				)
			}
			return author, slug, nil
		}
	}
	return "", "", fmt.Errorf("catalog has no eligible provider offering")
}
