package providers

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type credentialRunClient func(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error)

func (f credentialRunClient) ListModels(ctx context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
	return f(ctx, material)
}

func TestConcurrentObservationsKeepTheirPreflightCredentials(t *testing.T) {
	ctx := t.Context()
	firstReady := make(chan struct{})
	releaseFirst := make(chan struct{})
	defer close(releaseFirst)
	var resolutions atomic.Int64
	var clients atomic.Int64
	versions := make(chan string, 2)
	resolver := sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		version := strconv.FormatInt(resolutions.Add(1), 10)
		return sources.NewProviderCredentialMaterial(
			catalogs.ProviderCredentialProfile{ID: "profile-" + catalogs.ProviderCredentialProfileID(version), Primitive: catalogs.ProviderAuthenticationNone},
			nil, sources.ProviderCredentialMetadata{Version: version},
		), nil
	})
	source := newTestSource(newProviderSet(providerForTest("scoped-provider")),
		WithCredentialResolver(resolver),
		WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
			number := clients.Add(1)
			if number == 1 {
				close(firstReady)
				select {
				case <-releaseFirst:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return credentialRunClient(func(_ context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
				versions <- strconv.FormatInt(number, 10) + ":" + material.Version() + ":" + string(material.Profile().ID)
				return []catalogs.Model{{ID: "model", Name: "Model"}}, nil
			}), nil
		}),
	)
	firstResult := make(chan error, 1)
	go func() {
		observation, _, err := source.ObserveAttempts(ctx)
		if err == nil {
			err = observation.Validate()
		}
		firstResult <- err
	}()
	select {
	case <-firstReady:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	second, _, err := source.ObserveAttempts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := <-versions; got != "2:2:profile-2" {
		t.Errorf("second observation used %s", got)
	}
	releaseFirst <- struct{}{}
	select {
	case err := <-firstResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if got := <-versions; got != "1:1:profile-1" {
		t.Errorf("first observation changed its preflight credentials: %s", got)
	}
	if got := resolutions.Load(); got != 2 {
		t.Errorf("resolved credentials %d times, want once per observation", got)
	}
}

func TestFailedConcurrentCredentialResolutionDoesNotPoisonPeer(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"absent", &pkgerrors.AuthenticationError{Provider: "scoped-provider", Message: "no credential"}},
		{"invalid", &pkgerrors.ConfigError{Component: "credential", Message: "invalid reference"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			firstReady := make(chan struct{})
			releaseFirst := make(chan struct{})
			defer close(releaseFirst)
			var resolutions atomic.Int64
			var requests atomic.Int64
			source := newTestSource(newProviderSet(providerForTest("scoped-provider")),
				WithCredentialResolver(sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
					if resolutions.Add(1) > 1 {
						return sources.ProviderCredentialMaterial{}, test.err
					}
					return sources.NewProviderCredentialMaterial(catalogs.ProviderCredentialProfile{ID: "unauthenticated", Primitive: catalogs.ProviderAuthenticationNone}, nil, sources.ProviderCredentialMetadata{Version: "first"}), nil
				})),
				WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
					close(firstReady)
					select {
					case <-releaseFirst:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					return credentialRunClient(func(_ context.Context, material sources.ProviderCredentialMaterial) ([]catalogs.Model, error) {
						requests.Add(1)
						if material.Version() != "first" {
							t.Errorf("request lost its preflight credential")
						}
						return []catalogs.Model{{ID: "model", Name: "Model"}}, nil
					}), nil
				}),
			)
			type result struct {
				observation sources.Observation
				attempts    []sources.ProviderAttempt
				err         error
			}
			firstResult := make(chan result, 1)
			go func() {
				observation, attempts, err := source.ObserveAttempts(ctx)
				firstResult <- result{observation, attempts, err}
			}()
			select {
			case <-firstReady:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			second, attempts, err := source.ObserveAttempts(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if second.Status != sources.ObservationStatusDegraded || len(attempts) != 1 || attempts[0].Requested {
				t.Fatalf("failed credential resolution sent a request or concealed degradation: %+v", attempts)
			}
			releaseFirst <- struct{}{}
			select {
			case first := <-firstResult:
				if first.err != nil {
					t.Fatal(first.err)
				}
				if first.observation.Status != sources.ObservationStatusSucceeded || len(first.attempts) != 1 || first.attempts[0].Outcome != sources.ProviderOutcomeSucceeded {
					t.Fatalf("peer credential failure changed the first observation: %+v", first.attempts)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if requests.Load() != 1 || resolutions.Load() != 2 {
				t.Fatalf("requests=%d resolutions=%d", requests.Load(), resolutions.Load())
			}
		})
	}
}
