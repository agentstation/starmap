package runtime

import (
	stderrors "errors"
	"io/fs"
	"reflect"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestAuthorityRuntimeExplicitTransitionAcceptsNewAuthority(t *testing.T) {
	for _, scenario := range []struct {
		field      string
		freshState bool
	}{
		{"authority", false}, {"policy", false}, {"authority", true}, {"policy", true},
	} {
		name := scenario.field
		if scenario.freshState {
			name += "/fresh-state"
		}
		t.Run(name, func(t *testing.T) {
			previous, options := authorityRuntimeFixture(t)
			old := transitionGeneration(t, previous.receipt, "previous-authority", 10, activeAlias("author/previous"))
			previous.receipt.Head = old.Manifest.AuthorityHead
			previous.replies = []SourceRead{aliasRead(old)}
			store := &retentionRejectingStore{Memory: storage.NewMemory()}
			options = append(options, WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store)))
			connected := openTestRuntime(t, options...)
			if _, err := connected.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !connected.AllowsNewAttempt() {
				t.Fatal("previous authority was not ready")
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}

			next, _ := authorityRuntimeFixture(t)
			if scenario.field == "authority" {
				next.receipt.Head.AuthorityID = "replacement-enterprise"
			} else {
				next.receipt.Head.PolicyID = "replacement-policy"
			}
			replacement := transitionGeneration(t, next.receipt, "replacement-authority", 1)
			next.receipt.Head = replacement.Manifest.AuthorityHead
			next.replies = []SourceRead{aliasRead(replacement)}
			next.permissionErr = fs.ErrNotExist
			options = append(options, WithSource(next), WithSourceAuthority(next.receipt.Head.AuthorityID, next.receipt.Head.PolicyID))
			if scenario.freshState {
				options = append(options, WithStateDirectory(privateRuntimeDirectory(t)))
			}
			transition := openTestRuntime(t, options...)
			if transition.Catalog() == nil || transition.AllowsNewAttempt() || transition.AllowsCatalogAttempt(old.Manifest.AuthorityHead) {
				t.Fatal("changed configuration reused the previous authority approval")
			}
			if err := transition.Close(); err != nil {
				t.Fatal(err)
			}
			transition = openTestRuntime(t, options...)
			if transition.AllowsNewAttempt() {
				t.Fatal("restart approved the incomplete authority transition")
			}

			next.permissionMu.Lock()
			next.permissionErr = nil
			next.permissionMu.Unlock()
			store.reject.Store(true)
			if _, err := transition.RefreshSource(t.Context()); !stderrors.Is(err, fs.ErrPermission) {
				t.Fatalf("replacement did not reach the rejecting store: %v", err)
			}
			retained, err := store.Current(t.Context())
			if err != nil || !reflect.DeepEqual(retained, old) || transition.AllowsNewAttempt() {
				t.Fatalf("failed transition changed retained state or permitted inference: %v", err)
			}
			store.reject.Store(false)
			if err := transition.Close(); err != nil {
				t.Fatal(err)
			}
			transition = openTestRuntime(t, options...)
			if transition.AllowsNewAttempt() {
				t.Fatal("recovered failed transition approved the previous catalog")
			}
			if _, err := transition.RefreshSource(t.Context()); err != nil {
				t.Fatalf("explicit authority transition: %v", err)
			}
			if !transition.AllowsNewAttempt() || transition.AllowsCatalogAttempt(old.Manifest.AuthorityHead) {
				t.Fatal("replacement authority failed to replace the admission context")
			}
			accepted, err := transition.Client().CurrentGeneration(t.Context())
			if err != nil || !reflect.DeepEqual(accepted, replacement) {
				t.Fatalf("replacement authority did not preserve its exact generation: %v", err)
			}
			if len(transition.Catalog().CanonicalAliases().Records()) != 0 {
				t.Fatal("previous authority aliases entered the new authority catalog")
			}
			if err := transition.Close(); err != nil {
				t.Fatal(err)
			}
			next.permissionMu.Lock()
			next.permissionErr = fs.ErrNotExist
			next.permissionMu.Unlock()
			restarted := openTestRuntime(t, options...)
			if !restarted.AllowsNewAttempt() || restarted.State().AuthorityHead != replacement.Manifest.AuthorityHead {
				t.Fatal("completed transition lost the retained replacement approval")
			}
		})
	}
}

func transitionGeneration(t *testing.T, receipt catalogs.CatalogPermissionEnvelope, id string, sequence uint64, aliases ...catalogs.CanonicalAlias) catalogs.Generation {
	t.Helper()
	generation := aliasGeneration(t, id, aliases...)
	receipt.Head.GenerationID = id
	receipt.Head.Sequence = sequence
	receipt.Head.PayloadChecksum = generation.Manifest.Payload.Checksum
	generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
	generation.Manifest.AuthorityHead = receipt.Head
	return generation
}
