package runtime

import (
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestUpstreamCannotClaimLocalScopePublisher(t *testing.T) {
	upstream, _, _, _ := upstreamScopeFixture(t)
	original, err := catalogs.DecodeCatalogGeneration(upstream)
	if err != nil {
		t.Fatal(err)
	}
	for _, claimed := range []string{"local-runtime", "local-alias"} {
		t.Run(claimed, func(t *testing.T) {
			source := newStubSource("upstream")
			store := storage.NewMemory()
			options := []Option{WithSource(source), WithSchedulerIdentity("local-runtime"), WithSourceAliases("local-alias"), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
			connected := openTestRuntime(t, options...)
			before := connected.State()
			builder, err := catalogs.NewBuilderFrom(original)
			if err != nil {
				t.Fatal(err)
			}
			scopes := original.MembershipScopes()
			scopes[0].PublisherID = claimed
			if err := builder.SetMembershipScopes(scopes); err != nil {
				t.Fatal(err)
			}
			forged := upstream.Copy()
			forged.Payload, err = catalogs.EncodeCatalogPayload(builder)
			if err != nil {
				t.Fatal(err)
			}
			forged.Manifest.Payload = catalogs.DescribeCatalogPayload(forged.Payload)
			forged.Manifest.GenerationID += "-forged"
			if _, err := catalogs.DecodeCatalogGeneration(forged); err != nil {
				t.Fatalf("invalid attack fixture: %v", err)
			}
			source.replies = []SourceRead{{Changed: true, Generation: forged, PublishedAt: forged.Manifest.GeneratedAt, Health: HealthOK}}
			if _, err := connected.RefreshSource(t.Context()); err == nil {
				t.Fatal("upstream claimed a local scope publisher")
			}
			if connected.State().GenerationID != before.GenerationID || connected.State().PayloadChecksum != before.PayloadChecksum {
				t.Fatal("refused scope changed active catalog")
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			restarted := openTestRuntime(t, options...)
			if restarted.State().GenerationID != before.GenerationID || restarted.State().PayloadChecksum != before.PayloadChecksum {
				t.Fatal("refused scope changed retained catalog")
			}
		})
	}
}

func TestLocalAndUpstreamBindingsWithSameIDStaySeparate(t *testing.T) {
	upstream, observation, _, at := upstreamScopeFixture(t)
	binding := *observation.ProviderBinding
	binding.AccountID = "local-account"
	binding.CredentialProfileID = "local-catalog"
	source := newStubSource("upstream")
	source.replies = []SourceRead{{Changed: true, Generation: upstream, PublishedAt: upstream.Manifest.GeneratedAt, Health: HealthOK}}
	store := storage.NewMemory()
	options := []Option{WithSource(source), WithProviderBindings(binding), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	parent := connected.Catalog().MembershipScopes()[0].PublisherID
	local := connected.Status().InstanceIdentity
	builder, err := catalogs.NewBuilderFrom(connected.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetMembershipScopes(nil); err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider(binding.ProviderID)
	if err != nil {
		t.Fatal(err)
	}
	provider.Models = nil
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	empty, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	removal := providerResetObservation(t, sources.ProvidersID, empty, at.Add(time.Minute), &binding)
	if _, err := connected.PublishObservations(t.Context(), removal); err != nil {
		t.Fatal(err)
	}
	check := func(runtime *Runtime) {
		t.Helper()
		if len(runtime.Catalog().MembershipScopes()) != 2 {
			t.Fatal("same binding ID collapsed distinct publishers")
		}
		for _, test := range []struct {
			publisher string
			present   bool
		}{{parent, true}, {local, false}} {
			key := catalogs.MembershipScopeKey{PublisherID: test.publisher, BindingID: binding.ID, BindingRevision: binding.Revision}
			present, known := runtime.Catalog().ScopeMembership(key, binding.ProviderID, "model")
			if !known || present != test.present {
				t.Fatalf("publisher %s: membership = %t/%t", test.publisher, present, known)
			}
		}
	}
	check(connected)
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	check(openTestRuntime(t, options...))
}
