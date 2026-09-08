package config_test

import (
	"context"
	stderrors "errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

func TestPublicOptionsApplyAliasesAndCoalescing(t *testing.T) {
	for _, aliases := range []string{"load-balancer", ""} {
		t.Run("aliases="+aliases, func(t *testing.T) {
			parsed, err := config.Parse(map[string]string{
				config.Source: "embedded", config.SourceAliases: aliases,
				config.AcquisitionEnabled: "false", config.SourcePollInterval: "0s",
				config.CoalesceWindow: "17ms", config.StateDirectory: filepath.Join(t.TempDir(), "runtime"),
			})
			if err != nil {
				t.Fatal(err)
			}
			windows := make(chan time.Duration, 1)
			options := append(parsed.Options(), runtime.WithSource(aliasSource{}), runtime.WithAcquirer(windowAcquirer{windows}),
				runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())))
			connected, err := runtime.Open(t.Context(), options...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := connected.Close(); err != nil {
					t.Error(err)
				}
			})
			_, err = connected.RefreshSource(t.Context())
			var conflict *errors.ConflictError
			if aliases != "" {
				if !stderrors.As(err, &conflict) || conflict.Actual != aliases {
					t.Fatal("the public alias option did not reject its source cycle")
				}
			} else if err != nil {
				t.Fatalf("explicit empty aliases refused an unrelated source: %v", err)
			}
			if _, err := connected.Sync(t.Context()); err != nil {
				t.Fatal(err)
			}
			select {
			case window := <-windows:
				if window != 17*time.Millisecond {
					t.Fatal("the runtime did not pass the configured coalescing bound to acquisition")
				}
			default:
				t.Fatal("the runtime did not call acquisition")
			}
		})
	}
}

type windowAcquirer struct{ windows chan<- time.Duration }

func (a windowAcquirer) AcquireProviders(_ context.Context, request runtime.AcquisitionRequest) (runtime.AcquisitionResult, error) {
	a.windows <- request.CoalesceWindow
	return runtime.AcquisitionResult{}, nil
}

type aliasSource struct{}

func (aliasSource) Identity() string { return "upstream" }
func (aliasSource) Read(context.Context) (runtime.SourceRead, error) {
	return runtime.SourceRead{Chain: []runtime.SourceHop{{Identity: "load-balancer"}}}, nil
}

func TestDescriptorDefaultsAndNamedValuesParse(t *testing.T) {
	for _, descriptor := range config.Descriptors() {
		t.Run(descriptor.Key, func(t *testing.T) {
			if descriptor.Description == "" {
				t.Fatal("the setting lacks its operator description")
			}
			if descriptor.Default != "" || descriptor.AllowEmpty {
				if _, err := config.Parse(map[string]string{descriptor.Name: descriptor.Default}); err != nil {
					t.Fatalf("advertised default fails parsing: %v", err)
				}
			}
			for _, value := range descriptor.AllowedValues {
				if _, err := config.Parse(map[string]string{descriptor.Name: value}); err != nil {
					t.Fatalf("advertised name fails parsing: %v", err)
				}
			}
		})
	}
}

func TestSourceReplacementPreservesIndependentBasePolicy(t *testing.T) {
	resolved, err := config.Resolve(config.Layer{Name: "flags", Values: map[string]string{config.Source: "embedded"}})
	if err != nil {
		t.Fatal(err)
	}
	options := []runtime.Option{
		runtime.WithSourceAliases("load-balancer"), runtime.WithSourcePollInterval(0),
		runtime.WithAcquisitionEnabled(false), runtime.WithStateDirectory(filepath.Join(t.TempDir(), "runtime")),
		runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())),
	}
	options = append(options, resolved.Config.Options()...)
	options = append(options, runtime.WithSource(aliasSource{}))
	connected, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	})
	_, err = connected.RefreshSource(t.Context())
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || conflict.Actual != "load-balancer" {
		t.Fatal("source replacement cleared an independent node alias from the host policy")
	}
}
